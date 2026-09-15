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
	"strconv"
	"strings"
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
// Driven by exactly one goroutine at a time (the collector's per-node
// poller owns it for its whole lifetime), so it needs no internal locking.
type Client struct {
	cfg    Config
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

// recordedHostKeyAlgorithms extracts the key algorithm(s) knownhosts has on
// record for a host from a "key changed" error, so ensure can retry
// preferring those. Our default HostKeyAlgorithms order (ED25519 first, see
// buildConfig) can pick a different key type than the one actually recorded
// for a given host, which knownhosts then reports as "changed" even though
// the real trusted key was never compared. Retrying only re-runs the same
// knownhosts check against whatever key gets negotiated, so it can't weaken
// verification -- it can only fix a false reject.
func recordedHostKeyAlgorithms(err error) []string {
	var keyErr *knownhosts.KeyError
	if !errors.As(err, &keyErr) || len(keyErr.Want) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(keyErr.Want))
	var algos []string
	for _, want := range keyErr.Want {
		t := want.Key.Type()
		if !seen[t] {
			seen[t] = true
			algos = append(algos, t)
		}
	}
	return algos
}

// authMethods collects every candidate key (from a running SSH agent and/or
// identity_file) into a single ssh.PublicKeys AuthMethod.
//
// Must NOT be split into separate AuthMethod entries per key source: the
// ssh package's auth loop dedupes config.Auth entries by RFC 4252 method
// name, so a second "publickey" entry is silently skipped once an earlier
// one has been tried -- even if that one offered zero usable keys (e.g. an
// agent with no identities loaded), which would make identity_file's key
// never get attempted at all.
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
			if _, _, _, _, pubErr := ssh.ParseAuthorizedKey(key); pubErr == nil {
				return nil, fmt.Errorf(
					"identity_file %s contains a public key, not a private key — point it at the "+
						"matching private key instead (usually the same filename without .pub): %w", path, err)
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

// defaultHostKeyAlgorithms is the fallback preference order for a host with
// nothing (yet) recorded in known_hosts. golang.org/x/crypto/ssh's own
// default order puts RSA/ECDSA ahead of ED25519; most modern sshd/ssh
// clients do the opposite, so match that instead.
var defaultHostKeyAlgorithms = []string{
	ssh.KeyAlgoED25519,
	ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521,
	ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSA,
}

// hostKeyAlgorithms puts preferred first (deduplicated), then fills in the
// rest of defaultHostKeyAlgorithms as a fallback.
func hostKeyAlgorithms(preferred []string) []string {
	if len(preferred) == 0 {
		return defaultHostKeyAlgorithms
	}
	seen := make(map[string]bool, len(preferred))
	algos := make([]string, 0, len(preferred)+len(defaultHostKeyAlgorithms))
	for _, a := range preferred {
		if !seen[a] {
			seen[a] = true
			algos = append(algos, a)
		}
	}
	for _, a := range defaultHostKeyAlgorithms {
		if !seen[a] {
			seen[a] = true
			algos = append(algos, a)
		}
	}
	return algos
}

// buildConfig assembles a fresh ssh.ClientConfig on every call, re-reading
// the identity file and ~/.ssh/known_hosts from disk. This is deliberately
// not cached: it lets a fix to known_hosts (or a rotated key) take effect
// on the next reconnect attempt without having to restart swtop.
func buildConfig(cfg Config, preferredHostKeyAlgos []string) (*ssh.ClientConfig, error) {
	methods, err := authMethods(cfg.IdentityFile)
	if err != nil {
		return nil, err
	}
	hkcb, err := hostKeyCallback()
	if err != nil {
		return nil, err
	}

	return &ssh.ClientConfig{
		User:              cfg.User,
		Auth:              methods,
		HostKeyCallback:   hkcb,
		HostKeyAlgorithms: hostKeyAlgorithms(preferredHostKeyAlgos),
		Timeout:           cfg.Timeout,
	}, nil
}

func (c *Client) dial(addr string, preferredHostKeyAlgos []string) (*ssh.Client, error) {
	cfg, err := buildConfig(c.cfg, preferredHostKeyAlgos)
	if err != nil {
		return nil, err
	}
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return conn, nil
}

func (c *Client) ensure() (*ssh.Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	addr := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))

	conn, err := c.dial(addr, nil)
	if err != nil {
		// See recordedHostKeyAlgorithms: retry once preferring whatever
		// algorithm(s) are actually on record for this host.
		if preferred := recordedHostKeyAlgorithms(err); len(preferred) > 0 {
			if retryConn, retryErr := c.dial(addr, preferred); retryErr == nil {
				conn, err = retryConn, nil
			}
		}
	}
	if err != nil {
		return nil, err
	}

	c.client = conn
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
	if c.client != nil {
		_ = c.client.Close()
		c.client = nil
	}
}

// Close releases the underlying connection, if any.
func (c *Client) Close() error {
	if c.client == nil {
		return nil
	}
	err := c.client.Close()
	c.client = nil
	return err
}
