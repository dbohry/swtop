package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func bar(label string, pct float64, width int, detail string) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	if width < 5 {
		width = 5
	}

	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	filledStr := gaugeFillStyle(pct).Render(strings.Repeat("█", filled))
	emptyStr := gaugeEmptyStyle.Render(strings.Repeat("░", width-filled))

	pctStr := fmt.Sprintf("%5.1f%%", pct)
	line := fmt.Sprintf("%-8s [%s%s] %s", label, filledStr, emptyStr, pctStr)
	if detail != "" {
		line += "  " + headerStyle.Render(detail)
	}
	return line
}

// barLabeled is like bar, but renders the label in an accent color and the
// percentage in the same severity color as the gauge fill, for a more
// polished per-metric look (used in the cluster overview panel).
func barLabeled(label string, accent lipgloss.Color, pct float64, width int, detail string) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	if width < 5 {
		width = 5
	}

	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	filledStr := gaugeFillStyle(pct).Render(strings.Repeat("█", filled))
	emptyStr := gaugeEmptyStyle.Render(strings.Repeat("░", width-filled))

	labelStr := lipgloss.NewStyle().Bold(true).Foreground(accent).Render(fmt.Sprintf("%-5s", label))
	pctStyle := gaugeFillStyle(pct).Bold(true)
	pctStr := pctStyle.Render(fmt.Sprintf("%5.1f%%", pct))

	line := fmt.Sprintf("%s [%s%s] %s", labelStr, filledStr, emptyStr, pctStr)
	if detail != "" {
		line += "  " + headerStyle.Render(detail)
	}
	return line
}
