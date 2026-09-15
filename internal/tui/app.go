// Package tui implements the swtop terminal UI: a consolidated
// cluster-wide view (the whole swarm as one machine) plus one detail view
// per node, navigated like tabs.
package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dbohry/swtop/internal/model"
)

type snapshotMsg model.ClusterSnapshot
type tickMsg time.Time

// Model is the bubbletea model driving the whole TUI.
type Model struct {
	snapshots <-chan model.ClusterSnapshot
	cluster   model.ClusterSnapshot

	activeTab int // 0 = cluster (consolidated) view, 1..N = node index+1
	sortByMem bool

	width, height int
}

// New builds a Model that reads cluster snapshots from ch.
func New(ch <-chan model.ClusterSnapshot) Model {
	return Model{snapshots: ch}
}

func waitForSnapshot(ch <-chan model.ClusterSnapshot) tea.Cmd {
	return func() tea.Msg {
		snap, ok := <-ch
		if !ok {
			return nil
		}
		return snapshotMsg(snap)
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForSnapshot(m.snapshots), tickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case snapshotMsg:
		m.cluster = model.ClusterSnapshot(msg)
		if m.activeTab > len(m.cluster.Nodes) {
			m.activeTab = 0
		}
		return m, waitForSnapshot(m.snapshots)

	case tickMsg:
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % (len(m.cluster.Nodes) + 1)
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + len(m.cluster.Nodes) + 1) % (len(m.cluster.Nodes) + 1)
		case "s":
			m.sortByMem = !m.sortByMem
		default:
			if n, err := strconv.Atoi(msg.String()); err == nil && n >= 0 && n <= len(m.cluster.Nodes) {
				m.activeTab = n
			}
		}
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(m.renderTabs())
	b.WriteString("\n")

	if m.activeTab == 0 {
		b.WriteString(m.renderCluster())
	} else if idx := m.activeTab - 1; idx < len(m.cluster.Nodes) {
		b.WriteString(m.renderNode(m.cluster.Nodes[idx]))
	}

	b.WriteString("\n")
	sortLabel := "cpu%"
	if m.sortByMem {
		sortLabel = "mem"
	}
	b.WriteString(footerStyle.Render(fmt.Sprintf(
		"tab/←→: switch view   1-%d: jump to node   s: sort by %s   q: quit   updated %s",
		len(m.cluster.Nodes), sortLabel, m.cluster.UpdatedAt.Format("15:04:05"),
	)))

	return b.String()
}

func (m Model) renderTabs() string {
	var parts []string
	label := "Cluster"
	if m.activeTab == 0 {
		parts = append(parts, tabActiveStyle.Render(label))
	} else {
		parts = append(parts, tabStyle.Render(label))
	}
	for i, n := range m.cluster.Nodes {
		label := n.Name
		if !n.Online {
			label += " ✗"
		}
		if m.activeTab == i+1 {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabStyle.Render(label))
		}
	}
	title := titleStyle.Render("swtop")
	sub := headerStyle.Render(fmt.Sprintf("  %d/%d nodes online", m.cluster.OnlineCount(), len(m.cluster.Nodes)))
	return title + sub + "\n" + strings.Join(parts, " ")
}

func (m Model) renderCluster() string {
	var b strings.Builder
	agg := m.cluster.Aggregate()

	barWidth := 30
	if m.width > 0 {
		barWidth = m.width/2 - 20
		if barWidth < 10 {
			barWidth = 10
		}
		if barWidth > 40 {
			barWidth = 40
		}
	}

	b.WriteString(sectionTitleStyle.Render("Cluster (consolidated as one machine)"))
	b.WriteString("\n")
	b.WriteString(bar("CPU", agg.CPUPercent, barWidth, fmt.Sprintf("%d cores", totalCores(m.cluster))))
	b.WriteString("\n")
	b.WriteString(bar("Mem", percentOf(agg.MemUsedKB, agg.MemTotalKB), barWidth, fmt.Sprintf("%s / %s", humanizeKB(agg.MemUsedKB), humanizeKB(agg.MemTotalKB))))
	b.WriteString("\n")
	b.WriteString(bar("Disk", percentOf(agg.DiskUsedKB, agg.DiskTotalKB), barWidth, fmt.Sprintf("%s / %s", humanizeKB(agg.DiskUsedKB), humanizeKB(agg.DiskTotalKB))))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%-8s ↓ %-12s ↑ %-12s  load %.2f / %.2f / %.2f\n", "Net", humanizeRate(agg.NetRxBytesPerSec), humanizeRate(agg.NetTxBytesPerSec), agg.Load1, agg.Load5, agg.Load15))

	b.WriteString(sectionTitleStyle.Render("Nodes"))
	b.WriteString("\n")
	cols := []column{
		{title: "NAME", width: 16},
		{title: "ROLE", width: 8},
		{title: "STATUS", width: 8},
		{title: "CPU%", width: 6, right: true},
		{title: "MEM%", width: 6, right: true},
		{title: "CONTAINERS", width: 10, right: true},
	}
	b.WriteString(renderHeader(cols))
	b.WriteString("\n")
	for _, n := range m.cluster.Nodes {
		status := "up"
		style := lipgloss.NewStyle()
		if !n.Online {
			status = "down"
			style = offlineText
		}
		row := renderRow(cols, []string{
			n.Name,
			roleOr(n.Role),
			status,
			fmt.Sprintf("%.1f", n.Host.CPUPercent),
			fmt.Sprintf("%.1f", percentOf(n.Host.MemUsedKB, n.Host.MemTotalKB)),
			strconv.Itoa(len(n.Containers)),
		})
		b.WriteString(style.Render(row))
		b.WriteString("\n")
	}

	svcs := m.cluster.ServiceAggregates()
	if len(svcs) > 0 {
		sort.Slice(svcs, func(i, j int) bool {
			if m.sortByMem {
				return svcs[i].MemUsageBytes > svcs[j].MemUsageBytes
			}
			return svcs[i].CPUPercent > svcs[j].CPUPercent
		})
		b.WriteString(sectionTitleStyle.Render("Services"))
		b.WriteString("\n")
		scols := []column{
			{title: "NAME", width: 28},
			{title: "REPLICAS", width: 8, right: true},
			{title: "NODES", width: 6, right: true},
			{title: "CPU%", width: 8, right: true},
			{title: "MEM", width: 10, right: true},
		}
		b.WriteString(renderHeader(scols))
		b.WriteString("\n")
		for _, s := range svcs {
			b.WriteString(renderRow(scols, []string{
				s.Name,
				strconv.Itoa(s.Replicas),
				strconv.Itoa(len(s.Nodes)),
				fmt.Sprintf("%.1f", s.CPUPercent),
				humanizeBytes(s.MemUsageBytes),
			}))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderNode(n model.NodeSnapshot) string {
	var b strings.Builder

	header := fmt.Sprintf("%s  (%s)  role=%s", n.Name, n.Address, roleOr(n.Role))
	b.WriteString(sectionTitleStyle.Render(header))
	b.WriteString("\n")

	if !n.Online {
		b.WriteString(errStyle.Render("node unreachable"))
		if n.Err != "" {
			b.WriteString(": " + n.Err)
		}
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(headerStyle.Render(fmt.Sprintf("uptime %s   last update %s", humanizeUptime(n.Host.Uptime), n.UpdatedAt.Format("15:04:05"))))
	b.WriteString("\n")

	barWidth := 24
	if m.width > 0 {
		barWidth = m.width/3 - 20
		if barWidth < 8 {
			barWidth = 8
		}
		if barWidth > 30 {
			barWidth = 30
		}
	}

	// Per-core CPU bars, wrapped into rows of up to 4.
	perRow := 4
	for i := 0; i < len(n.Host.PerCoreCPU); i += perRow {
		end := i + perRow
		if end > len(n.Host.PerCoreCPU) {
			end = len(n.Host.PerCoreCPU)
		}
		var cells []string
		for j := i; j < end; j++ {
			cells = append(cells, bar(fmt.Sprintf("Core%d", j), n.Host.PerCoreCPU[j], barWidth, ""))
		}
		b.WriteString(strings.Join(cells, "  "))
		b.WriteString("\n")
	}
	if len(n.Host.PerCoreCPU) == 0 {
		b.WriteString(bar("CPU", n.Host.CPUPercent, barWidth*2, ""))
		b.WriteString("\n")
	}

	b.WriteString(bar("Mem", percentOf(n.Host.MemUsedKB, n.Host.MemTotalKB), barWidth*2, fmt.Sprintf("%s / %s", humanizeKB(n.Host.MemUsedKB), humanizeKB(n.Host.MemTotalKB))))
	b.WriteString("\n")
	if n.Host.SwapTotalKB > 0 {
		b.WriteString(bar("Swap", percentOf(n.Host.SwapUsedKB, n.Host.SwapTotalKB), barWidth*2, fmt.Sprintf("%s / %s", humanizeKB(n.Host.SwapUsedKB), humanizeKB(n.Host.SwapTotalKB))))
		b.WriteString("\n")
	}
	b.WriteString(bar("Disk", percentOf(n.Host.DiskUsedKB, n.Host.DiskTotalKB), barWidth*2, fmt.Sprintf("%s / %s", humanizeKB(n.Host.DiskUsedKB), humanizeKB(n.Host.DiskTotalKB))))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%-8s ↓ %-12s ↑ %-12s  load %.2f / %.2f / %.2f\n", "Net", humanizeRate(n.Host.NetRxBytesPerSec), humanizeRate(n.Host.NetTxBytesPerSec), n.Host.Load1, n.Host.Load5, n.Host.Load15))

	b.WriteString(sectionTitleStyle.Render(fmt.Sprintf("Containers (%d)", len(n.Containers))))
	b.WriteString("\n")

	containers := make([]model.Container, len(n.Containers))
	copy(containers, n.Containers)
	sort.Slice(containers, func(i, j int) bool {
		if m.sortByMem {
			return containers[i].MemUsageBytes > containers[j].MemUsageBytes
		}
		return containers[i].CPUPercent > containers[j].CPUPercent
	})

	cols := []column{
		{title: "NAME", width: 24},
		{title: "SERVICE", width: 18},
		{title: "CPU%", width: 6, right: true},
		{title: "MEM", width: 10, right: true},
		{title: "NET IO", width: 18, right: true},
		{title: "BLOCK IO", width: 18, right: true},
		{title: "PIDS", width: 5, right: true},
		{title: "STATUS", width: 16},
	}
	b.WriteString(renderHeader(cols))
	b.WriteString("\n")
	for _, c := range containers {
		b.WriteString(renderRow(cols, []string{
			c.Name,
			c.ServiceName,
			fmt.Sprintf("%.1f", c.CPUPercent),
			humanizeBytes(c.MemUsageBytes),
			fmt.Sprintf("%s / %s", humanizeBytes(c.NetRxBytes), humanizeBytes(c.NetTxBytes)),
			fmt.Sprintf("%s / %s", humanizeBytes(c.BlockReadBytes), humanizeBytes(c.BlockWriteBytes)),
			strconv.Itoa(c.PIDs),
			c.Status,
		}))
		b.WriteString("\n")
	}

	return b.String()
}

func roleOr(role string) string {
	if role == "" {
		return "-"
	}
	return role
}

func totalCores(c model.ClusterSnapshot) int {
	n := 0
	for _, node := range c.Nodes {
		if node.Online {
			n += len(node.Host.PerCoreCPU)
		}
	}
	return n
}
