package tui

import (
	"strings"
)

const colGap = 2

const minColWidth = 6

type column struct {
	title      string
	width      int
	right      bool
	flexWeight int
}

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
	out := make([]column, len(cols))
	copy(out, cols)

	switch {
	case extra > 0:
		for i, c := range out {
			if c.flexWeight > 0 {
				out[i].width = c.width + extra*c.flexWeight/totalWeight
			}
		}
	case extra < 0:
		// Too narrow even at baseline: shrink the flexible columns
		// (proportionally to their weight) instead of silently
		// overflowing the available width.
		deficit := -extra
		for i, c := range out {
			if c.flexWeight == 0 || deficit == 0 {
				continue
			}
			shrink := deficit * c.flexWeight / totalWeight
			if maxShrink := c.width - minColWidth; shrink > maxShrink {
				shrink = maxShrink
			}
			if shrink > 0 {
				out[i].width = c.width - shrink
			}
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
