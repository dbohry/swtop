// Package config loads swtop's YAML configuration: SSH defaults and the
// inventory of swarm nodes to monitor.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration so it can be parsed from YAML strings like "2s".
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

// SSHDefaults are applied to any node that doesn't override them.
type SSHDefaults struct {
	User         string   `yaml:"user"`
	IdentityFile string   `yaml:"identity_file"`
	Port         int      `yaml:"port"`
	Timeout      Duration `yaml:"timeout"`
}

// NodeConfig describes one swarm node reachable over SSH.
type NodeConfig struct {
	Name         string `yaml:"name"`
	Address      string `yaml:"address"`
	Role         string `yaml:"role"` // "manager" or "worker" (informational only)
	User         string `yaml:"user"`
	IdentityFile string `yaml:"identity_file"`
	Port         int    `yaml:"port"`
}

// Config is the root swtop configuration.
type Config struct {
	PollInterval Duration     `yaml:"poll_interval"`
	SSH          SSHDefaults  `yaml:"ssh"`
	Nodes        []NodeConfig `yaml:"nodes"`
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

// Load reads and validates a config file from path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := defaults()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if len(cfg.Nodes) == 0 {
		return nil, fmt.Errorf("config must define at least one node under 'nodes'")
	}

	for i := range cfg.Nodes {
		n := &cfg.Nodes[i]
		if n.Name == "" {
			n.Name = n.Address
		}
		if n.Address == "" {
			return nil, fmt.Errorf("node %q is missing an address", n.Name)
		}
		if n.User == "" {
			n.User = cfg.SSH.User
		}
		if n.Port == 0 {
			n.Port = cfg.SSH.Port
		}
		if n.IdentityFile == "" {
			n.IdentityFile = cfg.SSH.IdentityFile
		}
	}

	if cfg.PollInterval.AsDuration() <= 0 {
		cfg.PollInterval = Duration(2 * time.Second)
	}

	return &cfg, nil
}
