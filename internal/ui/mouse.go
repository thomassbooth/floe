package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// part is what piece of a window the mouse is over.
type part int

const (
	partContent part = iota
	partTitle
	partEdge
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

// drag is a move or resize in progress.
type drag struct {
	window domain.WindowID
	// move is set when dragging by the title bar; otherwise edges are being
	// dragged.
	move   bool
	offset domain.Point // mouse position within the frame when grabbed
	size   domain.Size  // frame size when grabbed
	edges  []domain.Direction
}

// minButtonsWidth is the narrowest frame that shows title bar buttons.
const minButtonsWidth = 14

// buttons returns the columns of a frame's maximize and close buttons, which
// sit at the right of its top border as " □ × ─╮".
func buttons(f domain.Rect) (maxCol, closeCol int, ok bool) {
	if f.W < minButtonsWidth {
		return 0, 0, false
	}
	corner := f.Right() - 1
	return corner - 5, corner - 3, true
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
		if maxCol, closeCol, ok := buttons(f); ok {
			switch {
			case x >= closeCol-1 && x <= closeCol+1:
				return target{window: w, part: partClose}, true
			case x >= maxCol-1 && x <= maxCol:
				return target{window: w, part: partMaximize}, true
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

func (m *Model) mousePress(msg tea.MouseClickMsg) {
	mouse := msg.Mouse()
	if m.switcher != nil {
		m.switcher = nil
		return
	}
	if mouse.Y == m.height-statusBarHeight {
		m.statusBarClick(mouse.X, mouse.Button)
		return
	}
	if mouse.Button != tea.MouseLeft {
		return
	}
	t, ok := m.targetAt(mouse.X, mouse.Y)
	if !ok {
		return
	}
	id := t.window.ID
	m.keys.Reset()
	m.backend.Apply(domain.FocusWindow{ID: id})

	switch t.part {
	case partClose:
		m.backend.Apply(domain.CloseWindow{ID: id})
	case partMaximize:
		m.backend.Apply(domain.ToggleMaximize{ID: id})
	case partTitle:
		f := t.window.Frame
		m.drag = &drag{
			window: id,
			move:   true,
			offset: domain.Point{X: mouse.X - f.X, Y: mouse.Y - f.Y},
			size:   f.Size(),
		}
	case partEdge:
		m.drag = &drag{window: id, edges: t.edges}
	}
}

// statusBarClick handles a click on the status bar. A left click on the
// session badge opens the session switcher and one on a window's tab
// focuses that window; a middle click on a tab closes the window.
func (m *Model) statusBarClick(x int, button tea.MouseButton) {
	if m.prompt != nil || x >= m.width {
		return // the prompt replaces the badge and tabs
	}
	if button == tea.MouseLeft && x < m.badgeWidth() {
		m.keys.Reset()
		m.switcher = newSwitcher(m.sessions)
		return
	}
	for _, tab := range m.statusTabs() {
		if x < tab.x0 || x >= tab.x1 {
			continue
		}
		switch button {
		case tea.MouseLeft:
			m.keys.Reset()
			m.backend.Apply(domain.FocusWindow{ID: tab.window.ID})
		case tea.MouseMiddle:
			m.backend.Apply(domain.CloseWindow{ID: tab.window.ID})
		}
		return
	}
}

// mouseMotion turns a drag into absolute commands, so a fast drag that
// skips cells still lands exactly under the mouse.
func (m *Model) mouseMotion(msg tea.MouseMotionMsg) {
	if m.drag == nil {
		return
	}
	mouse := msg.Mouse()
	d := m.drag
	if d.move {
		m.backend.Apply(domain.PlaceWindow{ID: d.window, Frame: domain.Rect{
			X: mouse.X - d.offset.X,
			Y: mouse.Y - d.offset.Y,
			W: d.size.W,
			H: d.size.H,
		}})
		return
	}
	for _, edge := range d.edges {
		pos := mouse.X
		switch edge {
		case domain.Right:
			pos = mouse.X + 1
		case domain.Up:
			pos = mouse.Y
		case domain.Down:
			pos = mouse.Y + 1
		}
		m.backend.Apply(domain.ResizeWindowEdge{ID: d.window, Edge: edge, Position: pos})
	}
}

func (m *Model) mouseRelease() {
	m.drag = nil
}
