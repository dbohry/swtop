package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dbohry/swtop/internal/model"
)

const defaultWidth = 80

const minViewportHeight = 3

type snapshotMsg model.ClusterSnapshot

// overview is one of the consolidated tabs shown before the per-host tabs.
type overview int

const (
	// overviewCluster consolidates only the Docker Swarm nodes.
	overviewCluster overview = iota
	// overviewAll consolidates every configured host, swarm nodes and plain
	// servers alike.
	overviewAll
)

type Model struct {
	snapshots <-chan model.ClusterSnapshot
	cluster   model.ClusterSnapshot

	activeTab int
	sortByMem bool

	width, height int

	viewport viewport.Model
}

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

func (m Model) Init() tea.Cmd {
	return waitForSnapshot(m.snapshots)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case snapshotMsg:
		m.cluster = model.ClusterSnapshot(msg)
		if m.activeTab >= m.numTabs() {
			m.activeTab = 0
		}
		cmd = waitForSnapshot(m.snapshots)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % m.numTabs()
			m.viewport.GotoTop()
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + m.numTabs()) % m.numTabs()
			m.viewport.GotoTop()
		case "s":
			m.sortByMem = !m.sortByMem
		case "c", "a":
			want := overviewCluster
			if msg.String() == "a" {
				want = overviewAll
			}
			for i, ov := range m.overviews() {
				if ov == want {
					m.activeTab = i
					m.viewport.GotoTop()
				}
			}
		default:
			if n, err := strconv.Atoi(msg.String()); err == nil && n >= 0 && n <= len(m.cluster.Nodes) {
				m.activeTab = 0
				if n > 0 {
					m.activeTab = len(m.overviews()) + n - 1
				}
				m.viewport.GotoTop()
			} else {
				m.viewport, cmd = m.viewport.Update(msg)
			}
		}
	}

	m.syncViewport()
	return m, cmd
}

// overviews returns the consolidated tabs to show. The Cluster tab only
// appears when swarm nodes are configured, and the All tab only when plain
// servers are configured (with swarm nodes alone it would duplicate Cluster).
func (m Model) overviews() []overview {
	if len(m.cluster.Nodes) == 0 {
		return []overview{overviewCluster}
	}
	hasSwarm := len(m.cluster.SwarmNodes().Nodes) > 0
	var out []overview
	if hasSwarm {
		out = append(out, overviewCluster)
	}
	if m.cluster.HasServers() {
		out = append(out, overviewAll)
	}
	return out
}

func (m Model) numTabs() int {
	return len(m.overviews()) + len(m.cluster.Nodes)
}

func (m Model) overviewSnapshot(ov overview) model.ClusterSnapshot {
	if ov == overviewCluster {
		return m.cluster.SwarmNodes()
	}
	return m.cluster
}

// hostNoun is what the configured machines are called in the UI: "node" for
// a pure swarm setup, "host" once plain servers are in the mix.
func (m Model) hostNoun() string {
	if m.cluster.HasServers() {
		return "host"
	}
	return "node"
}

func (m *Model) syncViewport() {
	header, body, footerLines := m.layout()

	width := m.width
	if width <= 0 {
		width = defaultWidth
	}
	height := m.height - strings.Count(header, "\n") - footerLines
	if height < minViewportHeight {
		height = minViewportHeight
	}

	m.viewport.Width = width
	m.viewport.Height = height
	m.viewport.SetContent(body)
}

func (m Model) View() string {
	header, _, _ := m.layout()
	return header + m.viewport.View() + "\n" + m.renderFooter()
}

func (m Model) effectiveWidth() int {
	if m.width > 0 {
		return m.width
	}
	return defaultWidth
}

func (m Model) layout() (header, body string, footerLines int) {
	width := m.effectiveWidth()

	header = m.renderTabs() + "\n"
	overviews := m.overviews()
	if m.activeTab < len(overviews) {
		ov := overviews[m.activeTab]
		header += m.renderOverviewHeader(ov)
		body = m.renderOverviewBody(ov)
	} else if idx := m.activeTab - len(overviews); idx < len(m.cluster.Nodes) {
		node := m.cluster.Nodes[idx]
		header += m.renderNodeHeader(node)
		if node.Online {
			body = m.renderNodeBody(node)
		}
	}
	header = clampLines(header, width)

	footerLines = 1
	if maxHeaderLines := m.height - minViewportHeight - footerLines; maxHeaderLines > 0 {
		header = capLines(header, maxHeaderLines)
	}

	return header, body, footerLines
}

func capLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n") + "\n"
}

func (m Model) renderFooter() string {
	sortLabel := "cpu%"
	if m.sortByMem {
		sortLabel = "mem"
	}
	jump := "0: overview"
	if len(m.overviews()) > 1 {
		jump = "c/a: cluster/all"
	}
	text := fmt.Sprintf(
		"tab/←→: switch view   %s   1-%d: jump to %s   ↑↓/pgup/pgdn: scroll   s: sort by %s   q: quit   updated %s",
		jump, len(m.cluster.Nodes), m.hostNoun(), sortLabel, m.cluster.UpdatedAt.Format("15:04:05"),
	)
	return footerStyle.Render(clampLines(text, m.effectiveWidth()))
}

func (m Model) renderTabs() string {
	var parts []string
	overviews := m.overviews()
	for i, ov := range overviews {
		label := "Cluster"
		if ov == overviewAll {
			label = "All"
		}
		if m.activeTab == i {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabStyle.Render(label))
		}
	}
	for i, n := range m.cluster.Nodes {
		label := n.Name
		if !n.Online {
			label += " ✗"
		}
		if m.activeTab == len(overviews)+i {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabStyle.Render(label))
		}
	}
	title := titleStyle.Render("swtop")
	sub := headerStyle.Render(fmt.Sprintf("  %d/%d %ss online", m.cluster.OnlineCount(), len(m.cluster.Nodes), m.hostNoun()))
	return title + sub + "\n" + strings.Join(parts, " ")
}

func (m Model) renderOverviewHeader(ov overview) string {
	snap := m.overviewSnapshot(ov)
	agg := snap.Aggregate()
	width := m.effectiveWidth()

	barWidth := clampBarWidth(width, 2, 20, 30, 10, 40)

	lines := []string{
		barLabeled("CPU", accentCPU, agg.CPUPercent, barWidth, fmt.Sprintf("%d cores", totalCores(snap))),
		barLabeled("Mem", accentMem, percentOf(agg.MemUsedKB, agg.MemTotalKB), barWidth, fmt.Sprintf("%s / %s", humanizeKB(agg.MemUsedKB), humanizeKB(agg.MemTotalKB))),
		barLabeled("Disk", accentDisk, percentOf(agg.DiskUsedKB, agg.DiskTotalKB), barWidth, fmt.Sprintf("%s / %s", humanizeKB(agg.DiskUsedKB), humanizeKB(agg.DiskTotalKB))),
		netLoadLineStyled(agg),
	}

	title := fmt.Sprintf("Cluster Overview · %d node%s consolidated", len(snap.Nodes), plural(len(snap.Nodes)))
	if ov == overviewAll {
		title = fmt.Sprintf("All Hosts · %d host%s consolidated", len(snap.Nodes), plural(len(snap.Nodes)))
	}
	return renderBox(title, colorBlue, width, lines) + "\n"
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (m Model) renderOverviewBody(ov overview) string {
	snap := m.overviewSnapshot(ov)
	var b strings.Builder
	width := m.effectiveWidth()
	inner := width - 4
	if inner < 1 {
		inner = 1
	}

	cols := fitColumns([]column{
		{title: "NAME", width: 16, flexWeight: 1},
		{title: "ROLE", width: 8},
		{title: "STATUS", width: 8},
		{title: "CPU%", width: 6, right: true},
		{title: "MEM%", width: 6, right: true},
		{title: "CONTAINERS", width: 10, right: true},
	}, inner)

	nodeLines := []string{renderHeader(cols)}
	for _, n := range snap.Nodes {
		status := "up"
		style := lipgloss.NewStyle()
		if !n.Online {
			status = "down"
			style = offlineText
		}
		containerCount := "-"
		if n.Docker {
			containerCount = strconv.Itoa(len(n.Containers))
		}
		row := renderRow(cols, []string{
			n.Name,
			roleOr(n.Role),
			status,
			fmt.Sprintf("%.1f", n.Host.CPUPercent),
			fmt.Sprintf("%.1f", percentOf(n.Host.MemUsedKB, n.Host.MemTotalKB)),
			containerCount,
		})
		nodeLines = append(nodeLines, style.Render(row))
	}
	boxTitle := "Nodes"
	if ov == overviewAll {
		boxTitle = "Hosts"
	}
	b.WriteString(renderBox(fmt.Sprintf("%s (%d/%d online)", boxTitle, snap.OnlineCount(), len(snap.Nodes)), colorGreen, width, nodeLines))

	svcs := snap.ServiceAggregates()
	if len(svcs) > 0 {
		sort.Slice(svcs, func(i, j int) bool {
			if m.sortByMem {
				return svcs[i].MemUsageBytes > svcs[j].MemUsageBytes
			}
			return svcs[i].CPUPercent > svcs[j].CPUPercent
		})
		scols := fitColumns([]column{
			{title: "NAME", width: 28, flexWeight: 1},
			{title: "REPLICAS", width: 8, right: true},
			{title: "NODES", width: 6, right: true},
			{title: "CPU%", width: 8, right: true},
			{title: "MEM", width: 10, right: true},
		}, inner)
		svcLines := []string{renderHeader(scols)}
		for _, s := range svcs {
			svcLines = append(svcLines, renderRow(scols, []string{
				s.Name,
				strconv.Itoa(s.Replicas),
				strconv.Itoa(len(s.Nodes)),
				fmt.Sprintf("%.1f", s.CPUPercent),
				humanizeBytes(s.MemUsageBytes),
			}))
		}
		b.WriteString("\n")
		sortLabel := "cpu%"
		if m.sortByMem {
			sortLabel = "mem"
		}
		b.WriteString(renderBox(fmt.Sprintf("Services (%d) · sorted by %s", len(svcs), sortLabel), colorPurple, width, svcLines))
	}

	return b.String()
}

func (m Model) renderNodeHeader(n model.NodeSnapshot) string {
	width := m.effectiveWidth()
	title := fmt.Sprintf("%s · %s · role=%s", n.Name, n.Address, roleOr(n.Role))

	if !n.Online {
		msg := "node unreachable"
		if n.Err != "" {
			msg += ": " + n.Err
		}
		return renderBox(title, colorRed, width, []string{errStyle.Render(msg)}) + "\n"
	}

	inner := width - 4
	if inner < 1 {
		inner = 1
	}

	var lines []string
	lines = append(lines, headerStyle.Render(fmt.Sprintf("uptime %s   last update %s", humanizeUptime(n.Host.Uptime), n.UpdatedAt.Format("15:04:05"))))

	barWidth := clampBarWidth(width, 3, 20, 24, 8, 30)

	perRow := coresPerRow(inner, barWidth)
	for i := 0; i < len(n.Host.PerCoreCPU); i += perRow {
		end := i + perRow
		if end > len(n.Host.PerCoreCPU) {
			end = len(n.Host.PerCoreCPU)
		}
		var cells []string
		for j := i; j < end; j++ {
			cells = append(cells, bar(fmt.Sprintf("Core%d", j), n.Host.PerCoreCPU[j], barWidth, ""))
		}
		lines = append(lines, strings.Join(cells, "  "))
	}
	if len(n.Host.PerCoreCPU) == 0 {
		lines = append(lines, barLabeled("CPU", accentCPU, n.Host.CPUPercent, barWidth*2, ""))
	}

	lines = append(lines, barLabeled("Mem", accentMem, percentOf(n.Host.MemUsedKB, n.Host.MemTotalKB), barWidth*2, fmt.Sprintf("%s / %s", humanizeKB(n.Host.MemUsedKB), humanizeKB(n.Host.MemTotalKB))))
	if n.Host.SwapTotalKB > 0 {
		lines = append(lines, barLabeled("Swap", accentSwap, percentOf(n.Host.SwapUsedKB, n.Host.SwapTotalKB), barWidth*2, fmt.Sprintf("%s / %s", humanizeKB(n.Host.SwapUsedKB), humanizeKB(n.Host.SwapTotalKB))))
	}
	lines = append(lines, barLabeled("Disk", accentDisk, percentOf(n.Host.DiskUsedKB, n.Host.DiskTotalKB), barWidth*2, fmt.Sprintf("%s / %s", humanizeKB(n.Host.DiskUsedKB), humanizeKB(n.Host.DiskTotalKB))))
	lines = append(lines, netLoadLineStyled(n.Host))

	return renderBox(title, colorBlue, width, lines) + "\n"
}

func (m Model) renderNodeBody(n model.NodeSnapshot) string {
	if !n.Docker {
		return m.renderProcessBody(n)
	}

	width := m.effectiveWidth()
	inner := width - 4
	if inner < 1 {
		inner = 1
	}

	containers := make([]model.Container, len(n.Containers))
	copy(containers, n.Containers)
	sort.Slice(containers, func(i, j int) bool {
		if m.sortByMem {
			return containers[i].MemUsageBytes > containers[j].MemUsageBytes
		}
		return containers[i].CPUPercent > containers[j].CPUPercent
	})

	cols := fitColumns([]column{
		{title: "NAME", width: 24, flexWeight: 3},
		{title: "SERVICE", width: 18, flexWeight: 2},
		{title: "CPU%", width: 6, right: true},
		{title: "MEM", width: 10, right: true},
		{title: "NET IO", width: 18, right: true},
		{title: "BLOCK IO", width: 18, right: true},
		{title: "PIDS", width: 5, right: true},
		{title: "STATUS", width: 16, flexWeight: 2},
	}, inner)

	lines := []string{renderHeader(cols)}
	for _, c := range containers {
		lines = append(lines, renderRow(cols, []string{
			c.Name,
			c.ServiceName,
			fmt.Sprintf("%.1f", c.CPUPercent),
			humanizeBytes(c.MemUsageBytes),
			fmt.Sprintf("%s / %s", humanizeBytes(c.NetRxBytes), humanizeBytes(c.NetTxBytes)),
			fmt.Sprintf("%s / %s", humanizeBytes(c.BlockReadBytes), humanizeBytes(c.BlockWriteBytes)),
			strconv.Itoa(c.PIDs),
			c.Status,
		}))
	}

	return renderBox(fmt.Sprintf("Containers (%d)", len(n.Containers)), colorCyan, width, lines)
}

func (m Model) renderProcessBody(n model.NodeSnapshot) string {
	width := m.effectiveWidth()
	inner := width - 4
	if inner < 1 {
		inner = 1
	}

	processes := make([]model.Process, len(n.Processes))
	copy(processes, n.Processes)
	sort.Slice(processes, func(i, j int) bool {
		if m.sortByMem {
			return processes[i].MemPercent > processes[j].MemPercent
		}
		return processes[i].CPUPercent > processes[j].CPUPercent
	})

	cols := fitColumns([]column{
		{title: "PID", width: 8, right: true},
		{title: "COMMAND", width: 24, flexWeight: 1},
		{title: "CPU%", width: 6, right: true},
		{title: "MEM%", width: 6, right: true},
	}, inner)

	lines := []string{renderHeader(cols)}
	for _, p := range processes {
		lines = append(lines, renderRow(cols, []string{
			strconv.Itoa(p.PID),
			p.Command,
			fmt.Sprintf("%.1f", p.CPUPercent),
			fmt.Sprintf("%.1f", p.MemPercent),
		}))
	}

	return renderBox(fmt.Sprintf("Processes (%d)", len(n.Processes)), colorCyan, width, lines)
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

func clampBarWidth(screenWidth, divisor, offset, def, min, max int) int {
	if screenWidth <= 0 {
		return def
	}
	w := screenWidth/divisor - offset
	if w < min {
		return min
	}
	if w > max {
		return max
	}
	return w
}

// netLoadLineStyled renders the network throughput/load line used inside
// the cluster and node overview panels, with directional colors.
func netLoadLineStyled(h model.HostStats) string {
	label := lipgloss.NewStyle().Bold(true).Foreground(accentNet).Render(fmt.Sprintf("%-5s", "Net"))
	down := lipgloss.NewStyle().Foreground(colorGreen).Render(fmt.Sprintf("↓ %-12s", humanizeRate(h.NetRxBytesPerSec)))
	up := lipgloss.NewStyle().Foreground(colorBlue).Render(fmt.Sprintf("↑ %-12s", humanizeRate(h.NetTxBytesPerSec)))
	load := headerStyle.Render(fmt.Sprintf("load %.2f / %.2f / %.2f", h.Load1, h.Load5, h.Load15))
	return fmt.Sprintf("%s %s %s  %s", label, down, up, load)
}

func coresPerRow(width, barWidth int) int {
	const overhead = 18
	cellWidth := barWidth + overhead
	n := (width + colGap) / (cellWidth + colGap)
	if n < 1 {
		n = 1
	}
	return n
}

func clampLines(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(line)
	}
	return strings.Join(lines, "\n")
}
