package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var boxBorderStyle = lipgloss.NewStyle().Foreground(colorDim)

// renderBox wraps lines in a rounded panel with a colored, embedded title,
// e.g.:
//
//	╭─ Title ──────────────╮
//	│ line one              │
//	│ line two               │
//	╰────────────────────────╯
func renderBox(title string, accent lipgloss.Color, width int, lines []string) string {
	if width < 8 {
		width = 8
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(accent)

	label := " " + title + " "
	headDashes := width - 2 - lipgloss.Width(label) - 1
	if headDashes < 0 {
		headDashes = 0
	}
	top := boxBorderStyle.Render("╭─") + titleStyle.Render(label) + boxBorderStyle.Render(strings.Repeat("─", headDashes)+"╮")
	bottom := boxBorderStyle.Render("╰" + strings.Repeat("─", width-2) + "╯")

	inner := width - 4
	if inner < 0 {
		inner = 0
	}

	var b strings.Builder
	b.WriteString(top)
	for _, l := range lines {
		b.WriteString("\n")
		b.WriteString(boxBorderStyle.Render("│ "))
		b.WriteString(padVisible(l, inner))
		b.WriteString(boxBorderStyle.Render(" │"))
	}
	b.WriteString("\n")
	b.WriteString(bottom)
	return b.String()
}

// padVisible pads or truncates s to an exact visible (ANSI-aware) width.
func padVisible(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w > width {
		return lipgloss.NewStyle().MaxWidth(width).Render(s)
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}
