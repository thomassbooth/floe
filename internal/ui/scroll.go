package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/thomassbooth/floe/internal/domain"
)

// wheelLines is how far one notch of the wheel scrolls.
const wheelLines = 3

// mouseWheel scrolls the window under the mouse back through its history.
// A full-screen program has no history, so it gets up and down arrows
// instead, as most terminals do.
func (m *Model) mouseWheel(msg tea.MouseWheelMsg) {
	mouse := msg.Mouse()
	w, ok := m.snap.WindowAt(mouse.X, mouse.Y)
	if !ok {
		return
	}
	if w.Content.Contains(mouse.X, mouse.Y) && m.wantsMouse(w) {
		c := &capture{window: w.ID, content: w.Content, button: mouseButtons[mouse.Button]}
		m.forward(c, domain.MouseWheel, mouse)
		return
	}
	st, ok := m.backend.State(w.Pane)
	if !ok {
		return
	}
	var up bool
	switch mouse.Button {
	case tea.MouseWheelUp:
		up = true
	case tea.MouseWheelDown:
	default:
		return
	}

	if st.AltScreen {
		key := domain.Key{Code: domain.KeyDown}
		if up {
			key.Code = domain.KeyUp
		}
		for range wheelLines {
			m.backend.Apply(domain.SendKey{Key: key, Window: w.ID})
		}
		return
	}

	lines := m.scroll[w.Pane]
	if up {
		lines = min(lines+wheelLines, st.Scrollback)
	} else {
		lines = max(lines-wheelLines, 0)
	}
	m.setScroll(w.Pane, lines, st.Scrollback)
}

// setScroll shows a pane lines up into its history, which was history
// lines long. Scrolling drops a selection in that pane, since its cells
// no longer line up.
func (m *Model) setScroll(pane domain.PaneID, lines, history int) {
	if s := m.selection; s != nil && s.pane == pane && m.scroll[pane] != lines {
		m.selection = nil
	}
	if lines <= 0 {
		delete(m.scroll, pane)
		delete(m.seen, pane)
		return
	}
	m.scroll[pane] = lines
	m.seen[pane] = history
}

// holdScrolledViews keeps scrolled windows looking at the same lines while
// new output pushes more lines into their history.
func (m *Model) holdScrolledViews() {
	for pane, lines := range m.scroll {
		st, ok := m.backend.State(pane)
		if !ok || st.AltScreen {
			m.setScroll(pane, 0, 0)
			continue
		}
		if grown := st.Scrollback - m.seen[pane]; grown > 0 {
			lines += grown
		}
		m.setScroll(pane, min(lines, st.Scrollback), st.Scrollback)
	}
}

// unscrollOnInput jumps a window back to the bottom when you type into it,
// like a terminal does.
func (m *Model) unscrollOnInput(cmd domain.Command) {
	var target domain.WindowID
	switch c := cmd.(type) {
	case domain.SendKey:
		target = c.Window
	case domain.Paste:
	default:
		return
	}
	for _, w := range m.snap.Windows {
		if (target == 0 && w.Focused) || w.ID == target {
			m.setScroll(w.Pane, 0, 0)
		}
	}
}
