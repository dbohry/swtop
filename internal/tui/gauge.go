package tui

import (
	"fmt"
	"strings"
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
