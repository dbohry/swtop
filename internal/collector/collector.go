package collector

import (
	"sync"
	"time"

	"github.com/dbohry/swtop/internal/config"
	"github.com/dbohry/swtop/internal/model"
	"github.com/dbohry/swtop/internal/probe"
	"github.com/dbohry/swtop/internal/sshx"
)

type Collector struct {
	interval time.Duration
	nodes    []config.NodeConfig

	mu    sync.Mutex
	state map[string]model.NodeSnapshot

	out chan model.ClusterSnapshot
}

func New(cfg *config.Config) *Collector {
	state := make(map[string]model.NodeSnapshot, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		state[n.Name] = model.NodeSnapshot{Name: n.Name, Address: n.Address, Role: n.Role}
	}
	return &Collector{
		interval: cfg.PollInterval.AsDuration(),
		nodes:    cfg.Nodes,
		state:    state,
		out:      make(chan model.ClusterSnapshot, 1),
	}
}

func (c *Collector) Snapshots() <-chan model.ClusterSnapshot { return c.out }

func (c *Collector) Run(stop <-chan struct{}) {
	var wg sync.WaitGroup
	for _, n := range c.nodes {
		wg.Add(1)
		go func(n config.NodeConfig) {
			defer wg.Done()
			c.pollNode(n, stop)
		}(n)
	}
	c.publish()
	wg.Wait()
}

func (c *Collector) pollNode(n config.NodeConfig, stop <-chan struct{}) {
	client := sshx.New(sshx.Config{
		Host:         n.Address,
		Port:         n.Port,
		User:         n.User,
		IdentityFile: n.IdentityFile,
		Timeout:      5 * time.Second,
	})
	defer client.Close()

	var prev *probe.Sample
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.runOnce(n, client, &prev)
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			c.runOnce(n, client, &prev)
		}
	}
}

func (c *Collector) runOnce(n config.NodeConfig, client *sshx.Client, prev **probe.Sample) {
	output, err := client.Run(probe.Script)
	if err != nil {
		c.mu.Lock()
		c.state[n.Name] = model.NodeSnapshot{
			Name: n.Name, Address: n.Address, Role: n.Role,
			Online: false, Err: err.Error(), UpdatedAt: time.Now(),
		}
		c.mu.Unlock()
		c.publish()
		*prev = nil
		return
	}

	host, containers, sample := probe.Parse(output, *prev)
	*prev = &sample
	for i := range containers {
		containers[i].Node = n.Name
	}

	c.mu.Lock()
	c.state[n.Name] = model.NodeSnapshot{
		Name: n.Name, Address: n.Address, Role: n.Role,
		Online: true, Host: host, Containers: containers, UpdatedAt: time.Now(),
	}
	c.mu.Unlock()
	c.publish()
}

func (c *Collector) publish() {
	c.mu.Lock()
	nodes := make([]model.NodeSnapshot, 0, len(c.nodes))
	for _, n := range c.nodes {
		nodes = append(nodes, c.state[n.Name])
	}
	c.mu.Unlock()

	snap := model.ClusterSnapshot{Nodes: nodes, UpdatedAt: time.Now()}
	select {
	case c.out <- snap:
	default:
		select {
		case <-c.out:
		default:
		}
		c.out <- snap
	}
}
