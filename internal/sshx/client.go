// Package sshx provides a small, reconnecting SSH command runner used to
// poll remote swarm nodes for host and Docker stats.
package sshx

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Config describes how to reach one remote host.
type Config struct {
	Host         string
	Port         int
	User         string
	IdentityFile string
	Timeout      time.Duration
}

// Client is a lazily-connected, auto-reconnecting SSH session runner.
// It is safe for concurrent use but commands against the same Client are
// serialized (each poller owns its own Client, so this only matters for
// Close/Ensure races).
type Client struct {
	cfg    Config
	mu     sync.Mutex
	client *ssh.Client
}

// New creates a Client. It does not connect until the first Run call.
func New(cfg Config) *Client {
	return &Client{cfg: cfg}
}

func expandHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

func hostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	known := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(known); err != nil {
		return nil, fmt.Errorf("known_hosts not found at %s (connect once with `ssh` manually to trust the host): %w", known, err)
	}
	cb, err := knownhosts.New(known)
	if err != nil {
		return nil, err
	}
	return wrapHostKeyCallback(cb, known), nil
}

// wrapHostKeyCallback turns knownhosts' terse errors into actionable ones:
// a changed key (possible MITM, or the host was reinstalled/reassigned)
// gets different guidance than a host that was simply never trusted.
func wrapHostKeyCallback(inner ssh.HostKeyCallback, knownHostsFile string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := inner(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}
		if len(keyErr.Want) == 0 {
			return fmt.Errorf(
				"host %s is not yet trusted in %s — run `ssh %s` once, verify the fingerprint, and accept it, then retry: %w",
				hostname, knownHostsFile, hostname, err)
		}
		return fmt.Errorf(
			"host key for %s has CHANGED since it was last trusted (previously recorded at %s) — this can "+
				"mean the host was reinstalled or its IP was reassigned to a different machine, or it can mean "+
				"a man-in-the-middle attack. Verify the new key's fingerprint with the node's owner/console "+
				"first; if it's expected, remove the stale entry (`ssh-keygen -R %s -f %s`) and run `ssh %s` "+
				"once to accept and trust the new key, then retry: %w",
			hostname, keyErr.Want[0].String(), hostname, knownHostsFile, hostname, err)
	}
}

// authMethods collects every candidate key (from a running SSH agent and/or
// identity_file) into a single ssh.PublicKeys AuthMethod.
//
// This must NOT be split into separate AuthMethod entries per key source:
// the ssh package's client-side auth loop dedupes config.Auth entries by
// RFC 4252 method name, so a second "publickey" entry is silently skipped
// once any earlier "publickey" entry has been tried — even if that earlier
// one offered zero usable keys (e.g. an agent with no identities loaded).
// That would make the identity_file key never get attempted at all.
func authMethods(identityFile string) ([]ssh.AuthMethod, error) {
	var signers []ssh.Signer

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			if agentSigners, err := agent.NewClient(conn).Signers(); err == nil {
				signers = append(signers, agentSigners...)
			}
		}
	}

	if identityFile != "" {
		path := expandHome(identityFile)
		key, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading identity file %s: %w", path, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			if strings.HasSuffix(path, ".pub") {
				return nil, fmt.Errorf(
					"identity_file %s looks like a public key, but it must point at the matching "+
						"private key (same name without .pub): %w", path, err)
			}
			return nil, fmt.Errorf("parsing identity file %s: %w", path, err)
		}
		signers = append(signers, signer)
	}

	if len(signers) == 0 {
		return nil, fmt.Errorf("no SSH auth available: set identity_file or run an ssh-agent with SSH_AUTH_SOCK")
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signers...)}, nil
}

// buildConfig assembles a fresh ssh.ClientConfig on every call, re-reading
// the identity file and ~/.ssh/known_hosts from disk. This is deliberately
// not cached: it lets a fix to known_hosts (or a rotated key) take effect
// on the next reconnect attempt without having to restart swtop.
func buildConfig(cfg Config) (*ssh.ClientConfig, error) {
	methods, err := authMethods(cfg.IdentityFile)
	if err != nil {
		return nil, err
	}
	hkcb, err := hostKeyCallback()
	if err != nil {
		return nil, err
	}

	return &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            methods,
		HostKeyCallback: hkcb,
		// golang.org/x/crypto/ssh's own default order puts RSA/ECDSA ahead of
		// ED25519. A server offering multiple host key types (the sshd
		// default) then negotiates a different key than a real ssh/ssh-keyscan
		// client would, and knownhosts reports a false "key changed" against
		// the ED25519 line that's actually recorded. Prefer ED25519 first to
		// match what ssh_host_*_key setups and modern OpenSSH clients do.
		HostKeyAlgorithms: []string{
			ssh.KeyAlgoED25519,
			ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521,
			ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSA,
		},
		Timeout: cfg.Timeout,
	}, nil
}

func (c *Client) ensure() (*ssh.Client, error) {
	c.mu.Lock()
	existing := c.client
	c.mu.Unlock()
	if existing != nil {
		return existing, nil
	}

	cfg, err := buildConfig(c.cfg)
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(c.cfg.Host, fmt.Sprintf("%d", c.cfg.Port))
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	c.mu.Lock()
	c.client = conn
	c.mu.Unlock()
	return conn, nil
}

// Run executes cmd on the remote host and returns combined stdout.
// On any connection-level failure the underlying SSH connection is dropped
// so the next call reconnects from scratch.
func (c *Client) Run(cmd string) (string, error) {
	conn, err := c.ensure()
	if err != nil {
		return "", err
	}

	session, err := conn.NewSession()
	if err != nil {
		c.drop()
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	session.Stdout = &stdout
	if err := session.Run(cmd); err != nil {
		if _, ok := err.(*ssh.ExitError); !ok {
			c.drop()
		}
		return stdout.String(), err
	}
	return stdout.String(), nil
}

func (c *Client) drop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client != nil {
		_ = c.client.Close()
		c.client = nil
	}
}

// Close releases the underlying connection, if any.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return nil
	}
	err := c.client.Close()
	c.client = nil
	return err
}
