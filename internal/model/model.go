package model

import "time"

type HostStats struct {
	CPUPercent float64
	PerCoreCPU []float64

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

type Container struct {
	ID          string
	Name        string
	Image       string
	ServiceName string
	Status      string
	Node        string

	CPUPercent float64

	MemUsageBytes uint64
	MemLimitBytes uint64

	NetRxBytes uint64
	NetTxBytes uint64

	BlockReadBytes  uint64
	BlockWriteBytes uint64

	PIDs int
}

type Process struct {
	PID        int
	Command    string
	CPUPercent float64
	MemPercent float64
}

type NodeSnapshot struct {
	Name    string
	Address string
	Role    string

	Docker bool

	Online bool
	Err    string

	Host       HostStats
	Containers []Container
	Processes  []Process

	UpdatedAt time.Time
}

type ClusterSnapshot struct {
	Nodes     []NodeSnapshot
	UpdatedAt time.Time
}

func (c ClusterSnapshot) OnlineCount() int {
	n := 0
	for _, node := range c.Nodes {
		if node.Online {
			n++
		}
	}
	return n
}

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

	if totalCores > 0 {
		agg.CPUPercent = totalBusy / totalCores * 100
	}

	agg.Load1 /= online
	agg.Load5 /= online
	agg.Load15 /= online

	return agg
}

type ServiceAggregate struct {
	Name          string
	Replicas      int
	Nodes         map[string]bool
	CPUPercent    float64
	MemUsageBytes uint64
}

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
