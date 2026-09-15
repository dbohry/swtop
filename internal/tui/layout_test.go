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
