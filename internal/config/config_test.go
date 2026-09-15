package config

import (
	"os"
	"testing"
)

func TestServersSectionLoads(t *testing.T) {
	yaml := `
poll_interval: 2s
ssh:
  user: pi
  identity_file: ~/.ssh/id_rsa
  port: 22

servers:
  - name: server-1
    address: 10.0.1.1
  - name: server-2
    address: 10.0.1.2

nodes:
  - name: saturn
    address: 192.168.178.10
    role: manager
`
	f, err := os.CreateTemp("", "swtop-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Nodes) != 1 || !cfg.Nodes[0].Docker {
		t.Fatalf("expected 1 docker-enabled node, got %+v", cfg.Nodes)
	}
	if len(cfg.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %+v", cfg.Servers)
	}
	for _, s := range cfg.Servers {
		if s.Docker {
			t.Errorf("server %q should not be docker-enabled", s.Name)
		}
		if s.User != "pi" || s.Port != 22 {
			t.Errorf("server %q did not inherit ssh defaults: %+v", s.Name, s)
		}
	}
}

func TestDuplicateNameAcrossNodesAndServers(t *testing.T) {
	yaml := `
nodes:
  - name: dup
    address: 10.0.0.1
servers:
  - name: dup
    address: 10.0.0.2
`
	f, err := os.CreateTemp("", "swtop-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	if _, err := Load(f.Name()); err == nil {
		t.Fatal("expected error for duplicate name across nodes/servers")
	}
}
