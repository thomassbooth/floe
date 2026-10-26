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
	notice        string

	// Err is set when the session ended because of a failure.
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
			if cmd, ok := m.keys.Handle(k); ok {
				m.backend.Apply(cmd)
			}
		}

	case tea.PasteMsg:
		m.backend.Apply(domain.Paste{Text: msg.Content})

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if mouse.Button == tea.MouseLeft {
			if w, ok := m.snap.WindowAt(mouse.X, mouse.Y); ok {
				m.keys.Reset()
				m.backend.Apply(domain.FocusWindow{ID: w.ID})
			}
		}

	case eventMsg:
		switch ev := msg.event.(type) {
		case domain.LayoutChanged:
			m.snap = ev.Snapshot
		case domain.CommandFailed:
			m.notice = ev.Err.Error()
		case domain.SessionEnded:
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
