package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
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

// click remembers a click, to spot double and triple clicks.
type click struct {
	at    time.Time
	x, y  int
	count int
}

// startSelection begins selecting at p in a window: a plain click starts
// a drag, a double click takes the word there and a triple click the line.
func (m *Model) startSelection(w domain.WindowView, p domain.Point) {
	now := time.Now()
	count := 1
	if prev := m.lastClick; prev.x == p.X && prev.y == p.Y && now.Sub(prev.at) < multiClickWindow {
		count = prev.count%3 + 1
	}
	m.lastClick = click{at: now, x: p.X, y: p.Y, count: count}

	at := domain.Point{X: p.X - w.Content.X, Y: p.Y - w.Content.Y}
	s := &selection{
		window:   w.ID,
		pane:     w.Pane,
		content:  w.Content,
		scroll:   m.scroll[w.Pane],
		anchor:   at,
		head:     at,
		dragging: true,
	}
	if count > 1 {
		if g, ok := m.backend.Grid(w.Pane, s.scroll); ok {
			s.whole = true
			if count == 2 {
				from, to := wordAt(g, at.X, at.Y)
				s.anchor.X, s.head.X = from, to
			} else {
				s.anchor.X, s.head.X = 0, lineEnd(g, at.Y)
			}
		}
	}
	m.selection = s
}

// extendTo moves the selection's free end to the screen cell p, kept
// inside the window's content.
func (s *selection) extendTo(p domain.Point) {
	s.head = domain.Point{
		X: min(max(p.X-s.content.X, 0), s.content.W-1),
		Y: min(max(p.Y-s.content.Y, 0), s.content.H-1),
	}
}

// finishSelection copies the selected text when the button is let go. The
// highlight stays until the next click or key.
func (m *Model) finishSelection() tea.Cmd {
	s := m.selection
	s.dragging = false
	if s.empty() {
		m.selection = nil
		return nil
	}
	g, ok := m.backend.Grid(s.pane, s.scroll)
	if !ok {
		return nil
	}
	text := selectedText(g, *s)
	if text == "" {
		return nil
	}
	m.info = fmt.Sprintf("copied %d characters", utf8.RuneCountInString(text))
	return tea.SetClipboard(text)
}

// dropStaleSelection forgets a selection whose window closed, moved, was
// resized or was scrolled, since its cells no longer line up.
func (m *Model) dropStaleSelection() {
	s := m.selection
	if s == nil {
		return
	}
	for _, w := range m.snap.Windows {
		if w.ID == s.window && w.Content == s.content && m.scroll[w.Pane] == s.scroll {
			return
		}
	}
	m.selection = nil
}
