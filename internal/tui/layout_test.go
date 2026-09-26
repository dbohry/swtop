package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dbohry/swtop/internal/model"
)

func layoutTestSnapshot(numCores, numContainers int) model.ClusterSnapshot {
	cores := make([]float64, numCores)
	for i := range cores {
		cores[i] = float64(i % 100)
	}
	containers := make([]model.Container, numContainers)
	for i := range containers {
		containers[i] = model.Container{
			ID: fmt.Sprintf("c%d", i), Name: fmt.Sprintf("service-%02d.1.xyz", i),
			ServiceName: fmt.Sprintf("service-%02d", i), Image: "nginx",
			Status:     "Up 10 days (healthy)",
			CPUPercent: float64(i), MemUsageBytes: uint64(i) * 1024 * 1024, PIDs: 3,
		}
	}
	nodes := make([]model.NodeSnapshot, 3)
	names := []string{"saturn", "jupiter", "mars"}
	for i := range nodes {
		nodes[i] = model.NodeSnapshot{
			Name: names[i], Address: "10.0.0.1", Role: "worker", Docker: true, Online: true,
			Host: model.HostStats{
				CPUPercent: 42.5, PerCoreCPU: cores,
				MemTotalKB: 16000000, MemUsedKB: 8000000,
				DiskTotalKB: 100000000, DiskUsedKB: 40000000,
				Load1: 0.5, Load5: 0.4, Load15: 0.3,
				NetRxBytesPerSec: 1024 * 500, NetTxBytesPerSec: 1024 * 200,
				Uptime: 5 * time.Hour,
			},
			Containers: containers,
			UpdatedAt:  time.Now(),
		}
	}
	return model.ClusterSnapshot{UpdatedAt: time.Now(), Nodes: nodes}
}

func layoutTestServerSnapshot(numCores, numProcesses int) model.ClusterSnapshot {
	cores := make([]float64, numCores)
	for i := range cores {
		cores[i] = float64(i % 100)
	}
	processes := make([]model.Process, numProcesses)
	for i := range processes {
		processes[i] = model.Process{
			PID: 1000 + i, Command: fmt.Sprintf("some-long-daemon-name-%02d", i),
			CPUPercent: float64(i), MemPercent: float64(i) / 2,
		}
	}
	snap := layoutTestSnapshot(numCores, 0)
	for i := range snap.Nodes {
		snap.Nodes[i].Docker = false
		snap.Nodes[i].Containers = nil
		snap.Nodes[i].Processes = processes
	}
	return snap
}

func TestServerLayoutFitsTerminal(t *testing.T) {
	sizes := []struct{ w, h int }{
		{120, 40}, {100, 30}, {80, 24}, {60, 20}, {40, 15},
	}

	for _, sz := range sizes {
		snap := layoutTestServerSnapshot(8, 20)
		tm := tea.Model(New(nil))
		tm, _ = tm.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		tm, _ = tm.Update(snapshotMsg(snap))

		checkView := func(label, out string) {
			t.Helper()
			for i, line := range strings.Split(out, "\n") {
				if w := lipgloss.Width(line); w > sz.w {
					t.Errorf("%s: line %d is %d cols wide (terminal is %d): %q", label, i, w, sz.w, line)
				}
			}
			if got := strings.Count(out, "\n") + 1; got != sz.h {
				t.Errorf("%s: rendered %d lines, want %d (terminal height)", label, got, sz.h)
			}
		}

		m := tm.(Model)
		tm2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
		m2 := tm2.(Model)
		checkView(fmt.Sprintf("server node w=%d h=%d", sz.w, sz.h), m2.View())

		if !strings.Contains(m2.View(), "Processes (20)") {
			t.Errorf("expected process panel with 20 entries, got:\n%s", m2.View())
		}
		if strings.Contains(m2.View(), "Containers (") {
			t.Errorf("plain server should not render a Containers panel, got:\n%s", m2.View())
		}
	}
}

func TestLayoutFitsTerminal(t *testing.T) {
	sizes := []struct{ w, h int }{
		{120, 40}, {100, 30}, {80, 24}, {60, 20}, {40, 15},
	}
	coreCounts := []int{1, 4, 8, 32}

	for _, sz := range sizes {
		for _, cores := range coreCounts {
			snap := layoutTestSnapshot(cores, 10)
			tm := tea.Model(New(nil))
			tm, _ = tm.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
			tm, _ = tm.Update(snapshotMsg(snap))

			checkView := func(label, out string) {
				t.Helper()
				for i, line := range strings.Split(out, "\n") {
					if w := lipgloss.Width(line); w > sz.w {
						t.Errorf("%s: line %d is %d cols wide (terminal is %d): %q", label, i, w, sz.w, line)
					}
				}
				if got := strings.Count(out, "\n") + 1; got != sz.h {
					t.Errorf("%s: rendered %d lines, want %d (terminal height)", label, got, sz.h)
				}
			}

			m := tm.(Model)
			checkView(fmt.Sprintf("cluster w=%d h=%d cores=%d", sz.w, sz.h, cores), m.View())

			tm2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
			m2 := tm2.(Model)
			checkView(fmt.Sprintf("node w=%d h=%d cores=%d", sz.w, sz.h, cores), m2.View())
		}
	}
}

func mixedSnapshot() model.ClusterSnapshot {
	snap := layoutTestSnapshot(4, 2)
	servers := layoutTestServerSnapshot(4, 3)
	servers.Nodes[0].Name = "web-01"
	snap.Nodes = append(snap.Nodes, servers.Nodes[0])
	return snap
}

func TestClusterViewExcludesServers(t *testing.T) {
	tm := tea.Model(New(nil))
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	tm, _ = tm.Update(snapshotMsg(mixedSnapshot()))
	m := tm.(Model)

	if got := m.overviews(); len(got) != 2 || got[0] != overviewCluster || got[1] != overviewAll {
		t.Fatalf("overviews = %v, want [Cluster All]", got)
	}

	cluster := m.View()
	if !strings.Contains(cluster, "3 nodes consolidated") || !strings.Contains(cluster, "Nodes (3/3 online)") {
		t.Errorf("cluster view should consolidate only the 3 swarm nodes, got:\n%s", cluster)
	}
	if strings.Contains(cluster, "│ web-01") {
		t.Errorf("cluster view should not list plain servers, got:\n%s", cluster)
	}

	tm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	all := tm.(Model).View()
	if !strings.Contains(all, "4 hosts consolidated") || !strings.Contains(all, "Hosts (4/4 online)") {
		t.Errorf("all view should consolidate every host, got:\n%s", all)
	}
	if !strings.Contains(all, "│ web-01") {
		t.Errorf("all view should list plain servers, got:\n%s", all)
	}

	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	if node := tm.(Model).View(); !strings.Contains(node, "Processes (3)") {
		t.Errorf("key 4 should jump to the 4th host (web-01), got:\n%s", node)
	}

	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if tm.(Model).activeTab != 0 {
		t.Errorf("key c should jump to the Cluster tab, activeTab = %d", tm.(Model).activeTab)
	}
}

func TestOverviewsForSingleKindSetups(t *testing.T) {
	swarm := tea.Model(New(nil))
	swarm, _ = swarm.Update(snapshotMsg(layoutTestSnapshot(4, 2)))
	if got := swarm.(Model).overviews(); len(got) != 1 || got[0] != overviewCluster {
		t.Errorf("swarm-only overviews = %v, want [Cluster]", got)
	}

	servers := tea.Model(New(nil))
	servers, _ = servers.Update(snapshotMsg(layoutTestServerSnapshot(4, 2)))
	if got := servers.(Model).overviews(); len(got) != 1 || got[0] != overviewAll {
		t.Errorf("servers-only overviews = %v, want [All]", got)
	}
}
