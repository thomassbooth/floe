package ui

import (
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"

	"github.com/thomassbooth/floe/internal/domain"
)

// multiClickWindow is how quickly clicks on one cell must follow each other
// to count as a double or triple click.
const multiClickWindow = 400 * time.Millisecond

// selection is text highlighted in a window, from where the mouse went down
// (anchor) to where it is now (head). Positions are cells of the window's
// content as currently shown, so the selection belongs to one scroll
// position and size of the window.
type selection struct {
	window       domain.WindowID
	pane         domain.PaneID
	content      domain.Rect
	scroll       int
	anchor, head domain.Point
	// dragging is set until the button is let go.
	dragging bool
	// whole is set for word and line selections, which are never empty.
	whole bool
}

// ordered returns the selection's ends in reading order.
func (s selection) ordered() (start, end domain.Point) {
	a, b := s.anchor, s.head
	if b.Y < a.Y || (b.Y == a.Y && b.X < a.X) {
		a, b = b, a
	}
	return a, b
}

// contains reports whether the content cell x, y is selected. Selections run
// like text: from the start to the end of each line in between.
func (s selection) contains(x, y int) bool {
	start, end := s.ordered()
	switch {
	case y < start.Y || y > end.Y:
		return false
	case start.Y == end.Y:
		return x >= start.X && x <= end.X
	case y == start.Y:
		return x >= start.X
	case y == end.Y:
		return x <= end.X
	}
	return true
}

// empty reports whether nothing has been selected: a plain click.
func (s selection) empty() bool {
	return s.anchor == s.head && !s.whole
}

// selectedText reads the selected cells from g, one line per row, without
// trailing spaces.
func selectedText(g domain.Grid, s selection) string {
	start, end := s.ordered()
	var lines []string
	for y := start.Y; y <= end.Y && y < g.Size.H; y++ {
		var b strings.Builder
		for x := range g.Size.W {
			c := g.At(x, y)
			if c.Width == 0 || !s.contains(x, y) {
				continue
			}
			if c.Content == "" {
				b.WriteByte(' ')
			} else {
				b.WriteString(c.Content)
			}
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	return strings.Join(lines, "\n")
}

// wordAt returns the ends of the word under x on row y: a run of cells
// that are not spaces or punctuation that usually separates words.
func wordAt(g domain.Grid, x, y int) (from, to int) {
	isWord := func(x int) bool {
		c := g.At(x, y).Content
		return c != "" && !strings.ContainsAny(c, " \t\"'`()[]{}<>|,;")
	}
	if !isWord(x) {
		return x, x
	}
	from, to = x, x
	for from > 0 && isWord(from-1) {
		from--
	}
	for to < g.Size.W-1 && isWord(to+1) {
		to++
	}
	return from, to
}

// lineEnd returns the last non-blank column of row y.
func lineEnd(g domain.Grid, y int) int {
	for x := g.Size.W - 1; x > 0; x-- {
		if c := g.At(x, y).Content; c != "" && c != " " {
			return x
		}
	}
	return 0
}

// drawSelection highlights the selected cells of a window.
func (m Model) drawSelection(scr uv.Screen, w domain.WindowView) {
	s := m.selection
	if s == nil || s.window != w.ID || s.empty() {
		return
	}
	area := w.Content
	for y := range area.H {
		for x := range area.W {
			if !s.contains(x, y) {
				continue
			}
			cell := scr.CellAt(area.X+x, area.Y+y)
			if cell == nil {
				continue
			}
			c := cell.Clone()
			c.Style.Bg, c.Style.Fg = m.theme.Accent, m.theme.BadgeFg
			c.Style.Attrs &^= uv.AttrReverse
			scr.SetCell(area.X+x, area.Y+y, c)
		}
	}
}
