package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorGreen  = lipgloss.Color("#5fd75f")
	colorYellow = lipgloss.Color("#d7d75f")
	colorRed    = lipgloss.Color("#d75f5f")
	colorBlue   = lipgloss.Color("#5fafd7")
	colorGray   = lipgloss.Color("#808080")
	colorWhite  = lipgloss.Color("#e4e4e4")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBlue)

	headerStyle = lipgloss.NewStyle().Foreground(colorGray)

	tabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000000")).Background(colorBlue).Padding(0, 1)
	tabStyle       = lipgloss.NewStyle().Foreground(colorGray).Padding(0, 1)

	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorWhite).MarginTop(1)

	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(colorGray)

	errStyle    = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	offlineText = lipgloss.NewStyle().Foreground(colorGray).Italic(true)

	footerStyle = lipgloss.NewStyle().Foreground(colorGray)

	// Gauge fill styles, precomputed once rather than per bar() call: a bar
	// is drawn for every core/mem/disk/swap gauge on every render, which
	// happens on every incoming snapshot.
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
