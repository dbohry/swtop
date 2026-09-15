package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration time.Duration

func (d Duration) AsDuration() time.Duration { return time.Duration(d) }

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

type SSHDefaults struct {
	User         string   `yaml:"user"`
	IdentityFile string   `yaml:"identity_file"`
	Port         int      `yaml:"port"`
	Timeout      Duration `yaml:"timeout"`
}

type NodeConfig struct {
	Name         string `yaml:"name"`
	Address      string `yaml:"address"`
	Role         string `yaml:"role"`
	User         string `yaml:"user"`
	IdentityFile string `yaml:"identity_file"`
	Port         int    `yaml:"port"`

	// Docker reports whether this host should be probed for container
	// stats. It is set by Load based on which list (nodes vs servers) the
	// entry came from, never read from YAML.
	Docker bool `yaml:"-"`
}

type Config struct {
	PollInterval Duration     `yaml:"poll_interval"`
	SSH          SSHDefaults  `yaml:"ssh"`
	Nodes        []NodeConfig `yaml:"nodes"`
	Servers      []NodeConfig `yaml:"servers"`
}

func defaults() Config {
	return Config{
		PollInterval: Duration(2 * time.Second),
		SSH: SSHDefaults{
			User:    "root",
			Port:    22,
			Timeout: Duration(5 * time.Second),
		},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := defaults()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if len(cfg.Nodes) == 0 && len(cfg.Servers) == 0 {
		return nil, fmt.Errorf("config must define at least one host under 'nodes' or 'servers'")
	}

	if err := applyDefaults(cfg.Nodes, cfg.SSH, true); err != nil {
		return nil, err
	}
	if err := applyDefaults(cfg.Servers, cfg.SSH, false); err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(cfg.Nodes)+len(cfg.Servers))
	for _, n := range cfg.Nodes {
		seen[n.Name] = true
	}
	for _, n := range cfg.Servers {
		if seen[n.Name] {
			return nil, fmt.Errorf("duplicate host name %q across 'nodes'/'servers'", n.Name)
		}
		seen[n.Name] = true
	}

	if cfg.PollInterval.AsDuration() <= 0 {
		cfg.PollInterval = Duration(2 * time.Second)
	}

	return &cfg, nil
}

func applyDefaults(nodes []NodeConfig, ssh SSHDefaults, docker bool) error {
	for i := range nodes {
		n := &nodes[i]
		if n.Name == "" {
			n.Name = n.Address
		}
		if n.Address == "" {
			return fmt.Errorf("host %q is missing an address", n.Name)
		}
		if n.User == "" {
			n.User = ssh.User
		}
		if n.Port == 0 {
			n.Port = ssh.Port
		}
		if n.IdentityFile == "" {
			n.IdentityFile = ssh.IdentityFile
		}
		n.Docker = docker
	}
	return nil
}
