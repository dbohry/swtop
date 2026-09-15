// Package model defines the data shapes shared between the collector and
// the TUI: per-node host stats, containers, and cluster-wide aggregates.
package model

import "time"

// HostStats is point-in-time (or rate-based) resource usage for one machine.
type HostStats struct {
	CPUPercent float64   // 0-100, averaged across cores
	PerCoreCPU []float64 // 0-100 per core

	MemTotalKB uint64
	MemUsedKB  uint64

	SwapTotalKB uint64
	SwapUsedKB  uint64

	Load1  float64
	Load5  float64
	Load15 float64

	DiskTotalKB uint64
	DiskUsedKB  uint64

	NetRxBytesPerSec float64
	NetTxBytesPerSec float64

	Uptime time.Duration
}

// Add accumulates another HostStats into a running cluster-wide total.
// CPUPercent/PerCoreCPU are intentionally left to the caller to average.
func (h *HostStats) Add(o HostStats) {
	h.MemTotalKB += o.MemTotalKB
	h.MemUsedKB += o.MemUsedKB
	h.SwapTotalKB += o.SwapTotalKB
	h.SwapUsedKB += o.SwapUsedKB
	h.DiskTotalKB += o.DiskTotalKB
	h.DiskUsedKB += o.DiskUsedKB
	h.NetRxBytesPerSec += o.NetRxBytesPerSec
	h.NetTxBytesPerSec += o.NetTxBytesPerSec
	h.Load1 += o.Load1
	h.Load5 += o.Load5
	h.Load15 += o.Load15
}

// Container is one Docker container's live resource usage, as reported by
// the local Docker engine on the node it runs on.
type Container struct {
	ID          string
	Name        string
	Image       string
	ServiceName string // from com.docker.swarm.service.name label, if any
	Status      string
	Node        string // set by the collector, not the remote command

	CPUPercent float64

	MemUsageBytes uint64
	MemLimitBytes uint64

	NetRxBytes uint64
	NetTxBytes uint64

	BlockReadBytes  uint64
	BlockWriteBytes uint64

	PIDs int
}

// NodeSnapshot is the latest known state of one swarm node.
type NodeSnapshot struct {
	Name    string
	Address string
	Role    string

	Online bool
	Err    string

	Host       HostStats
	Containers []Container

	UpdatedAt time.Time
}

// ClusterSnapshot is the latest known state of every configured node.
type ClusterSnapshot struct {
	Nodes     []NodeSnapshot
	UpdatedAt time.Time
}

// OnlineCount returns how many nodes are currently reachable.
func (c ClusterSnapshot) OnlineCount() int {
	n := 0
	for _, node := range c.Nodes {
		if node.Online {
			n++
		}
	}
	return n
}

// Aggregate sums/averages host stats across all online nodes, as if the
// whole cluster were one machine.
func (c ClusterSnapshot) Aggregate() HostStats {
	var agg HostStats
	var online, totalCores, totalBusy float64
	for _, node := range c.Nodes {
		if !node.Online {
			continue
		}
		online++
		agg.Add(node.Host)

		cores := float64(len(node.Host.PerCoreCPU))
		totalCores += cores
		totalBusy += node.Host.CPUPercent * cores / 100
	}
	if online == 0 {
		return agg
	}

	// CPUPercent is the utilization-weighted-by-core-count average, i.e.
	// total busy "core-percent" divided by total cores, matching how a
	// single machine's overall CPU% behaves.
	if totalCores > 0 {
		agg.CPUPercent = totalBusy / totalCores * 100
	}

	agg.Load1 /= online
	agg.Load5 /= online
	agg.Load15 /= online

	return agg
}

// ServiceAggregate is per-service resource usage summed across every task
// (container) of that service, cluster-wide.
type ServiceAggregate struct {
	Name          string
	Replicas      int
	Nodes         map[string]bool
	CPUPercent    float64
	MemUsageBytes uint64
}

// ServiceAggregates groups all containers cluster-wide by service name.
func (c ClusterSnapshot) ServiceAggregates() []ServiceAggregate {
	byName := map[string]*ServiceAggregate{}
	var order []string

	for _, node := range c.Nodes {
		for _, ct := range node.Containers {
			name := ct.ServiceName
			if name == "" {
				name = ct.Name
			}
			agg, ok := byName[name]
			if !ok {
				agg = &ServiceAggregate{Name: name, Nodes: map[string]bool{}}
				byName[name] = agg
				order = append(order, name)
			}
			agg.Replicas++
			agg.CPUPercent += ct.CPUPercent
			agg.MemUsageBytes += ct.MemUsageBytes
			agg.Nodes[node.Name] = true
		}
	}

	out := make([]ServiceAggregate, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out
}
