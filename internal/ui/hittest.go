package ui

import "github.com/thomassbooth/floe/internal/domain"

// part is what piece of a window the mouse is over.
type part int

const (
	partContent part = iota
	partTitle
	partEdge
	partMinimize
	partMaximize
	partClose
)

// target is the window and part of it under the mouse.
type target struct {
	window domain.WindowView
	part   part
	// edges lists the edges grabbed for partEdge: one, or two at a corner.
	edges []domain.Direction
}

// minButtonsWidth is the narrowest frame that shows title bar buttons.
const minButtonsWidth = 16

// titleButtons are the columns of a frame's minimize, maximize and close
// buttons, which sit at the right of its top border as " _ □ × ─╮".
type titleButtons struct {
	minimize, maximize, close int
}

// buttons returns where a frame's title bar buttons are, if it is wide
// enough to show them.
func buttons(f domain.Rect) (titleButtons, bool) {
	if f.W < minButtonsWidth {
		return titleButtons{}, false
	}
	corner := f.Right() - 1
	return titleButtons{minimize: corner - 7, maximize: corner - 5, close: corner - 3}, true
}

// targetAt finds what is under the cell x, y. The top corners resize, the
// rest of the top row moves the window, and the other border cells resize.
func (m Model) targetAt(x, y int) (target, bool) {
	w, ok := m.snap.WindowAt(x, y)
	if !ok {
		return target{}, false
	}
	f := w.Frame
	left, right := x == f.X, x == f.Right()-1
	top, bottom := y == f.Y, y == f.Bottom()-1

	if top {
		if b, ok := buttons(f); ok {
			switch {
			case x >= b.close-1 && x <= b.close+1:
				return target{window: w, part: partClose}, true
			case x >= b.maximize-1 && x <= b.maximize:
				return target{window: w, part: partMaximize}, true
			case x >= b.minimize-1 && x <= b.minimize:
				return target{window: w, part: partMinimize}, true
			}
		}
	}

	var edges []domain.Direction
	if left {
		edges = append(edges, domain.Left)
	}
	if right {
		edges = append(edges, domain.Right)
	}
	if bottom {
		edges = append(edges, domain.Down)
	}
	if top && (left || right) {
		edges = append(edges, domain.Up)
	}
	switch {
	case len(edges) > 0:
		return target{window: w, part: partEdge, edges: edges}, true
	case top:
		return target{window: w, part: partTitle}, true
	}
	return target{window: w, part: partContent}, true
}
