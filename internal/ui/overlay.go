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

// awaiter is an overlay that hears how its commands went, such as a prompt
// that stays open to show why a name was refused. Other overlays' failures
// show in the status bar.
type awaiter interface {
	overlay
	// result is given the result of a command the overlay sent while it is
	// still open. It reports whether the overlay is done.
	result(m *Model, r domain.Result) (done bool)
}

// resultMsg carries the result of a command an awaiter sent.
type resultMsg struct {
	from   awaiter
	result domain.Result
}

// keyToOverlay passes a key to the open overlay, closing it when it is
// done, and sends any command it gives. An awaiter that stays open waits
// for its command's result.
func (m *Model) keyToOverlay(k domain.Key) tea.Cmd {
	o := m.overlay
	cmd, done := o.key(m, k)
	if done {
		m.overlay = nil
	}
	a, waits := o.(awaiter)
	if cmd == nil || done || !waits {
		m.apply(cmd)
		return nil
	}
	reply := m.backend.Request(cmd)
	return func() tea.Msg { return resultMsg{from: a, result: <-reply} }
}

// takeResult hands a result to the overlay that asked for it, if it is
// still open, and otherwise shows a failure in the status bar.
func (m *Model) takeResult(msg resultMsg) {
	if m.overlay == overlay(msg.from) {
		if msg.from.result(m, msg.result) {
			m.overlay = nil
		}
		return
	}
	if msg.result.Err != nil {
		m.notice = msg.result.Err.Error()
	}
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
