package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/input"
)

// This file routes mouse events. A press decides what the drag after it
// does: feed a window's program (capture), select text (selection.go) or
// move or resize a window (drag.go). Motion and release go to whichever
// one the press started.

// capture sends a press, the drag after it and its release to the program
// in one window, even if the mouse leaves the window on the way.
type capture struct {
	window  domain.WindowID
	content domain.Rect
	button  domain.MouseButton
}

func (m *Model) mousePress(msg tea.MouseClickMsg) {
	mouse := msg.Mouse()
	p := domain.Point{X: mouse.X, Y: mouse.Y}
	m.selection, m.info = nil, ""
	if !m.pressOnOverlay(p) {
		return
	}
	if p.Y == m.statusBarRow() {
		m.statusBarClick(p.X, mouse.Button)
		return
	}
	t, ok := m.targetAt(p.X, p.Y)
	if !ok {
		return
	}
	id := t.window.ID
	if t.part == partContent && m.wantsMouse(t.window) {
		m.focus(id)
		m.capture = &capture{window: id, content: t.window.Content, button: mouseButtons[mouse.Button]}
		m.forward(m.capture, domain.MousePress, mouse)
		return
	}
	if mouse.Button != tea.MouseLeft {
		return
	}
	m.focus(id)

	// Moving and resizing only happen in arrange mode, or with ctrl held,
	// so a plain click never moves anything by accident.
	arranging := m.arranging() || mouse.Mod.Contains(tea.ModCtrl)
	switch t.part {
	case partContent:
		if !m.arranging() {
			m.startSelection(t.window, p)
		}
	case partClose:
		m.backend.Apply(domain.CloseWindow{ID: id})
	case partMaximize:
		m.backend.Apply(domain.ToggleMaximize{ID: id})
	case partMinimize:
		m.backend.Apply(domain.MinimizeWindow{ID: id})
	case partTitle:
		if arranging {
			m.drag = m.moveDrag(t.window, p)
		}
	case partEdge:
		if arranging {
			m.drag = resizeDrag(t.window, t.edges, p)
		}
	}
}

func (m *Model) mouseMotion(msg tea.MouseMotionMsg) {
	mouse := msg.Mouse()
	p := domain.Point{X: mouse.X, Y: mouse.Y}
	switch {
	case m.capture != nil:
		m.forward(m.capture, domain.MouseMove, mouse)
	case m.selection != nil && m.selection.dragging:
		m.selection.extendTo(p)
	case m.drag != nil:
		m.dragTo(p)
	}
}

// mouseRelease ends whatever the press started: a program's mouse capture,
// a text selection, which is copied, or a window drag, which may snap.
func (m *Model) mouseRelease(msg tea.MouseReleaseMsg) tea.Cmd {
	mouse := msg.Mouse()
	switch {
	case m.capture != nil:
		c := m.capture
		m.capture = nil
		m.forward(c, domain.MouseRelease, mouse)
	case m.selection != nil && m.selection.dragging:
		return m.finishSelection()
	case m.drag != nil:
		m.drop(domain.Point{X: mouse.X, Y: mouse.Y})
	}
	return nil
}

// focus gives a clicked window focus. A click also abandons a half-typed
// key sequence, such as a pressed prefix.
func (m *Model) focus(id domain.WindowID) {
	m.keys.Reset()
	m.backend.Apply(domain.FocusWindow{ID: id})
}

func (m Model) arranging() bool {
	return m.keys.Mode() == input.Arrange
}

// statusBarClick handles a click on the status bar. A left click on the
// session badge opens the session switcher and one on a window's tab
// focuses that window; a middle click on a tab closes the window.
func (m *Model) statusBarClick(x int, button tea.MouseButton) {
	if x >= m.width {
		return
	}
	if _, toggle := m.statusBar(m.width); button == tea.MouseLeft && toggle.ok && x >= toggle.x0 && x < toggle.x1 {
		m.keys.ToggleArrange()
		return
	}
	if button == tea.MouseLeft && x < m.badgeWidth() {
		m.keys.Reset()
		m.overlay = newSwitcher(m.sessions)
		return
	}
	for _, tab := range m.statusTabs() {
		if x < tab.x0 || x >= tab.x1 {
			continue
		}
		switch button {
		case tea.MouseLeft:
			m.focus(tab.window.ID)
		case tea.MouseMiddle:
			m.backend.Apply(domain.CloseWindow{ID: tab.window.ID})
		}
		return
	}
}

// wantsMouse reports whether a window's program takes mouse input. Arrange
// mode keeps the mouse for arranging.
func (m Model) wantsMouse(w domain.WindowView) bool {
	if m.arranging() {
		return false
	}
	st, ok := m.backend.State(w.Pane)
	return ok && st.Mouse != domain.MouseReportOff
}

// forward sends a mouse event to a captured window's program, at the
// position within its content, kept inside it.
func (m *Model) forward(c *capture, action domain.MouseAction, mouse tea.Mouse) {
	r := c.content
	m.backend.Apply(domain.SendMouse{Window: c.window, Event: domain.MouseEvent{
		Action: action,
		Button: c.button,
		X:      min(max(mouse.X-r.X, 0), max(r.W-1, 0)),
		Y:      min(max(mouse.Y-r.Y, 0), max(r.H-1, 0)),
		Mod:    toDomainMod(mouse.Mod),
	}})
}
