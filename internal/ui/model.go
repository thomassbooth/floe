// Package ui is the Bubble Tea client. It translates terminal input into
// domain commands and draws the session's snapshots. It is the only package
// that depends on Bubble Tea.
package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/input"
)

// frameInterval caps how often pane output triggers a redraw.
const frameInterval = time.Second / 60

// statusBarHeight is the number of rows below the windows.
const statusBarHeight = 1

// Backend is what the UI needs from a session.
type Backend interface {
	Apply(domain.Command)
	Events() <-chan domain.Event
	Changed() <-chan struct{}
	Grid(domain.PaneID) (domain.Grid, bool)
}

type (
	eventMsg        struct{ event domain.Event }
	eventsClosedMsg struct{}
	changedMsg      struct{}
	rearmChangedMsg struct{}
)

// Model is the Bubble Tea model for the multiplexer.
type Model struct {
	backend Backend
	keys    *input.Machine
	theme   Theme

	width, height int
	snap          domain.Snapshot
	sessions      domain.SessionsChanged
	notice        string

	// Overlays that take the keyboard while open; nil when closed.
	switcher *switcher
	prompt   *prompt

	// Err is set when the client was detached because of a failure.
	Err error
}

// New returns a model that drives backend with the given keymap.
func New(backend Backend, km input.Keymap) Model {
	return Model{
		backend: backend,
		keys:    input.NewMachine(km),
		theme:   DefaultTheme(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.waitEvent, m.waitChanged)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.backend.Apply(domain.SetArea{Size: domain.Size{
			W: msg.Width,
			H: max(msg.Height-statusBarHeight, 0),
		}})

	case tea.KeyPressMsg:
		m.notice = ""
		if k, ok := toDomainKey(tea.Key(msg)); ok {
			m.handleKey(k)
		}

	case tea.PasteMsg:
		m.backend.Apply(domain.Paste{Text: msg.Content})

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if m.switcher != nil {
			m.switcher = nil
		} else if mouse.Button == tea.MouseLeft {
			if w, ok := m.snap.WindowAt(mouse.X, mouse.Y); ok {
				m.keys.Reset()
				m.backend.Apply(domain.FocusWindow{ID: w.ID})
			}
		}

	case eventMsg:
		switch ev := msg.event.(type) {
		case domain.LayoutChanged:
			m.snap = ev.Snapshot
		case domain.SessionsChanged:
			m.sessions = ev
		case domain.CommandFailed:
			m.notice = ev.Err.Error()
		case domain.Detached:
			m.Err = ev.Err
			return m, tea.Quit
		}
		return m, m.waitEvent

	case eventsClosedMsg:
		return m, tea.Quit

	case changedMsg:
		// Redraw now, then wait a frame before listening again so a busy
		// pane cannot redraw more than once per frame.
		return m, tea.Tick(frameInterval, func(time.Time) tea.Msg { return rearmChangedMsg{} })

	case rearmChangedMsg:
		return m, m.waitChanged
	}
	return m, nil
}

// handleKey routes a key to the open overlay, or else to the key machine.
func (m *Model) handleKey(k domain.Key) {
	var cmd domain.Command
	switch {
	case m.prompt != nil:
		var done bool
		if cmd, done = m.prompt.handle(k); done {
			m.prompt = nil
		}
	case m.switcher != nil:
		var done bool
		if cmd, done = m.switcher.handle(k, m.sessions.Sessions); done {
			m.switcher = nil
		}
	default:
		res, ok := m.keys.Handle(k)
		if !ok {
			return
		}
		cmd = res.Command
		m.runAction(res.Action)
	}
	if cmd != nil {
		m.backend.Apply(cmd)
	}
}

func (m *Model) runAction(a input.Action) {
	switch a {
	case input.ActionSessionSwitcher:
		m.switcher = newSwitcher(m.sessions)
	case input.ActionRenameSession:
		m.prompt = newPrompt("rename session", m.snap.Name, func(name string) domain.Command {
			return domain.RenameSession{Name: name}
		})
	case input.ActionPrevSession:
		m.cycleSession(-1)
	case input.ActionNextSession:
		m.cycleSession(1)
	}
}

// cycleSession switches to the session delta places away in the list,
// wrapping around.
func (m *Model) cycleSession(delta int) {
	list := m.sessions.Sessions
	if len(list) < 2 {
		return
	}
	for i, info := range list {
		if info.ID == m.sessions.Current {
			next := list[(i+delta+len(list))%len(list)]
			m.backend.Apply(domain.SwitchSession{ID: next.ID})
			return
		}
	}
}

func (m Model) waitEvent() tea.Msg {
	ev, ok := <-m.backend.Events()
	if !ok {
		return eventsClosedMsg{}
	}
	return eventMsg{event: ev}
}

func (m Model) waitChanged() tea.Msg {
	<-m.backend.Changed()
	return changedMsg{}
}

// View implements tea.Model.
func (m Model) View() tea.View {
	content, cursor := m.render()
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "mux"
	v.Cursor = cursor
	return v
}
