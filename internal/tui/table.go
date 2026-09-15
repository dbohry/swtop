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

// fitColumns resizes flexible columns (flexWeight > 0) so the whole row
// fits within totalWidth, leaving fixed-width columns alone. Flexible
// columns share the leftover space proportionally to their weight and never
// shrink below minFlex. If totalWidth is unknown (<=0) or there are no
// flexible columns, cols is returned unchanged.
func fitColumns(cols []column, totalWidth, minFlex int) []column {
	if totalWidth <= 0 {
		return cols
	}

	fixed, totalWeight, numFlex := 0, 0, 0
	for _, c := range cols {
		if c.flexWeight > 0 {
			totalWeight += c.flexWeight
			numFlex++
		} else {
			fixed += c.width
		}
	}
	if totalWeight == 0 {
		return cols
	}

	extra := totalWidth - fixed - (len(cols)-1)*colGap
	if min := minFlex * numFlex; extra < min {
		extra = min
	}

	out := make([]column, len(cols))
	copy(out, cols)
	for i, c := range out {
		if c.flexWeight > 0 {
			w := extra * c.flexWeight / totalWeight
			if w < minFlex {
				w = minFlex
			}
			out[i].width = w
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
