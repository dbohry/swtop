package tui

import (
	"strings"
)

// colGap is the number of spaces rendered between adjacent columns.
const colGap = 2

type column struct {
	title      string
	width      int
	right      bool // right-align cell content
	flexWeight int  // >0 marks this column resizable by fitColumns
}

// fitColumns grows flexible columns (flexWeight > 0) to fill any width left
// over once every column has at least its declared width, sharing the
// extra proportionally to weight. A column's declared width is always its
// floor -- it only ever grows, never shrinks. If there's no extra width to
// give out (totalWidth unknown, or no wider than the columns already need),
// cols is returned unchanged.
func fitColumns(cols []column, totalWidth int) []column {
	if totalWidth <= 0 {
		return cols
	}

	baseline, totalWeight := 0, 0
	for _, c := range cols {
		baseline += c.width
		totalWeight += c.flexWeight
	}
	if totalWeight == 0 {
		return cols
	}

	extra := totalWidth - baseline - (len(cols)-1)*colGap
	if extra <= 0 {
		return cols
	}

	out := make([]column, len(cols))
	copy(out, cols)
	for i, c := range out {
		if c.flexWeight > 0 {
			out[i].width = c.width + extra*c.flexWeight/totalWeight
		}
	}
	return out
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
	return tableHeaderStyle.Render(strings.Join(parts, strings.Repeat(" ", colGap)))
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
	return strings.Join(parts, strings.Repeat(" ", colGap))
}
