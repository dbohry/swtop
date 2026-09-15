package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorGreen  = lipgloss.Color("#5fd75f")
	colorYellow = lipgloss.Color("#d7d75f")
	colorRed    = lipgloss.Color("#d75f5f")
	colorBlue   = lipgloss.Color("#5fafd7")
	colorCyan   = lipgloss.Color("#5fd7d7")
	colorPurple = lipgloss.Color("#af87d7")
	colorGray   = lipgloss.Color("#808080")
	colorDim    = lipgloss.Color("#5a5f6b")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBlue)

	headerStyle = lipgloss.NewStyle().Foreground(colorGray)

	tabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000000")).Background(colorBlue).Padding(0, 1)
	tabStyle       = lipgloss.NewStyle().Foreground(colorGray).Padding(0, 1)

	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(colorGray)

	errStyle    = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	offlineText = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	onlineDot   = lipgloss.NewStyle().Foreground(colorGreen)
	offlineDot  = lipgloss.NewStyle().Foreground(colorRed)

	footerStyle = lipgloss.NewStyle().Foreground(colorGray)

	// Accent colors for the cluster/node overview panels, one per metric.
	accentCPU  = colorGreen
	accentMem  = colorBlue
	accentDisk = colorPurple
	accentNet  = colorCyan
	accentSwap = colorYellow

	gaugeFillGreenStyle  = lipgloss.NewStyle().Foreground(colorGreen)
	gaugeFillYellowStyle = lipgloss.NewStyle().Foreground(colorYellow)
	gaugeFillRedStyle    = lipgloss.NewStyle().Foreground(colorRed)
	gaugeEmptyStyle      = lipgloss.NewStyle().Foreground(colorGray)
)

func gaugeFillStyle(pct float64) lipgloss.Style {
	switch {
	case pct >= 90:
		return gaugeFillRedStyle
	case pct >= 70:
		return gaugeFillYellowStyle
	default:
		return gaugeFillGreenStyle
	}
}
