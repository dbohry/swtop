package tui

import (
	"strings"
)

type column struct {
	title string
	width int
	right bool // right-align cell content
}

func padCell(s string, width int, right bool) string {
	s = truncate(s, width)
	if len(s) >= width {
		return s
	}
	pad := strings.Repeat(" ", width-len(s))
	if right {
		return pad + s
	}
	return s + pad
}

func renderHeader(cols []column) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = padCell(c.title, c.width, c.right)
	}
	return tableHeaderStyle.Render(strings.Join(parts, "  "))
}

func renderRow(cols []column, cells []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		parts[i] = padCell(cell, c.width, c.right)
	}
	return strings.Join(parts, "  ")
}
