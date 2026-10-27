package ui

import (
	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/thomassbooth/floe/internal/domain"
)

// overlay is something drawn over the windows that takes the keyboard while
// it is open, such as the session switcher, the theme picker or a prompt.
// At most one is open at a time.
type overlay interface {
	// key handles a key press. It returns a command for the session, if
	// any, and whether the overlay is done and should close.
	key(m *Model, k domain.Key) (cmd domain.Command, done bool)
	// press is told about a mouse press at p, anywhere on the screen. It
	// reports whether the overlay closes, and whether the press goes on to
	// whatever is under it.
	press(m *Model, p domain.Point) (closed, through bool)
	// draw draws the overlay over everything else. When it shows a cursor
	// of its own, or must hide the window's, it returns ok and the cursor,
	// which may be nil.
	draw(m Model, scr uv.Screen) (cursor *tea.Cursor, ok bool)
}

// keyToOverlay passes a key to the open overlay, closing it when it is
// done.
func (m *Model) keyToOverlay(k domain.Key) domain.Command {
	cmd, done := m.overlay.key(m, k)
	if done {
		m.overlay = nil
	}
	return cmd
}

// pressOnOverlay tells the open overlay about a mouse press and reports
// whether the press goes on to what is under it.
func (m *Model) pressOnOverlay(p domain.Point) bool {
	if m.overlay == nil {
		return true
	}
	closed, through := m.overlay.press(m, p)
	if closed {
		m.overlay = nil
	}
	return through
}
