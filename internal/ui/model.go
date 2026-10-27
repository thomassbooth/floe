// Package ui is the Bubble Tea client. It translates terminal input into
// domain commands and draws the session's snapshots. It is the only package
// that depends on Bubble Tea.
package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/input"
	"github.com/thomassbooth/floe/internal/theme"
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
	Grid(id domain.PaneID, scroll int) (domain.Grid, bool)
	State(id domain.PaneID) (domain.PaneState, bool)
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
	prefs   Preferences
	theme   theme.Theme
	// animate moves windows smoothly into a new layout, and anim is the
	// move under way, if any.
	animate bool
	anim    *animation
	// themeChosen is set once a theme was picked, now or in an earlier run,
	// so the terminal's background no longer decides it.
	themeChosen bool

	width, height int
	snap          domain.Snapshot
	sessions      domain.SessionsChanged
	notice        string

	// Overlays that take the keyboard while open; nil when closed.
	switcher *switcher
	picker   *themePicker
	prompt   *prompt
	// drag is the window move or resize in progress, if any.
	drag *drag
	// capture is the window whose program gets the mouse until the button
	// that was pressed in it is let go.
	capture *capture
	// selection is the highlighted text, if any, and lastClick the click
	// before, for spotting double and triple clicks.
	selection *selection
	lastClick click
	// info is a passing message for the status bar, like "copied".
	info string
	// scroll is how many lines each pane is scrolled up into its history,
	// and seen how long that history was when last checked, so new output
	// can be kept from moving a scrolled view.
	scroll map[domain.PaneID]int
	seen   map[domain.PaneID]int

	// Err is set when the client was detached because of a failure.
	Err error
	// DetachedFrom names the session that was showing when the user
	// detached; empty unless they did.
	DetachedFrom string
}

// Options are the client's choices, from the config files.
type Options struct {
	Keymap input.Keymap
	// Prefs keeps the theme; nil remembers nothing.
	Prefs Preferences
	// Animations moves windows smoothly when the layout changes.
	Animations bool
	// Notice, such as a problem with the config, shows in the status bar
	// until the first key.
	Notice string
}

// New returns a model that drives backend. The theme comes from the
// preferences when one was saved.
func New(backend Backend, o Options) Model {
	prefs := o.Prefs
	m := Model{
		backend: backend,
		keys:    input.NewMachine(o.Keymap),
		prefs:   prefs,
		animate: o.Animations,
		notice:  o.Notice,
		theme:   theme.MustLookup(theme.Default),
		scroll:  make(map[domain.PaneID]int),
		seen:    make(map[domain.PaneID]int),
	}
	if prefs != nil {
		if name, ok := prefs.Theme(); ok {
			if t, ok := theme.Lookup(name); ok {
				m.theme, m.themeChosen = t, true
			}
		}
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.waitEvent, m.waitChanged, tea.RequestBackgroundColor)
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

	case tea.BackgroundColorMsg:
		// Until a theme is picked, match the terminal: light themes for
		// light backgrounds.
		if !m.themeChosen && !msg.IsDark() {
			m.theme = theme.MustLookup(theme.DefaultLight)
		}

	case tea.KeyPressMsg:
		m.notice, m.info = "", ""
		m.selection = nil
		if k, ok := toDomainKey(tea.Key(msg)); ok {
			m.handleKey(k)
		}
		if m.DetachedFrom != "" {
			return m, tea.Quit
		}

	case tea.PasteMsg:
		m.backend.Apply(domain.Paste{Text: msg.Content})

	case tea.MouseClickMsg:
		m.mousePress(msg)

	case tea.MouseMotionMsg:
		m.mouseMotion(msg)

	case tea.MouseReleaseMsg:
		return m, m.mouseRelease(msg)

	case tea.MouseWheelMsg:
		m.mouseWheel(msg)

	case eventMsg:
		switch ev := msg.event.(type) {
		case domain.LayoutChanged:
			anim := m.setLayout(ev.Snapshot)
			m.dropStaleSelection()
			return m, tea.Batch(m.waitEvent, anim)
		case domain.SessionsChanged:
			m.sessions = ev
		case domain.ClipboardSet:
			return m, tea.Batch(m.waitEvent, tea.SetClipboard(ev.Text))
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
		m.holdScrolledViews()
		// Redraw now, then wait a frame before listening again so a busy
		// pane cannot redraw more than once per frame.
		return m, tea.Tick(frameInterval, func(time.Time) tea.Msg { return rearmChangedMsg{} })

	case rearmChangedMsg:
		return m, m.waitChanged

	case animTickMsg:
		return m, m.stepAnimation(msg.at)
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
	case m.picker != nil:
		m.handlePicker(k)
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
		m.unscrollOnInput(cmd)
	}
}

func (m *Model) runAction(a input.Action) {
	switch a {
	case input.ActionThemePicker:
		m.picker = newThemePicker(m.theme)
	case input.ActionNewWindow:
		// Half the whole terminal, centered in it. The session only knows
		// the area above the status bar, so the client works this out.
		w, h := m.width/2, m.height/2
		m.backend.Apply(domain.NewWindow{Frame: domain.Rect{
			X: (m.width - w) / 2,
			Y: (m.height - h) / 2,
			W: w,
			H: h,
		}})
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
	case input.ActionDetach:
		m.DetachedFrom = m.snap.Name
	}
}

// handlePicker previews the theme the picker is on, and keeps or reverts
// it when the picker closes.
func (m *Model) handlePicker(k domain.Key) {
	switch m.picker.handle(k) {
	case pickerOpen:
		m.theme = m.picker.current()
	case pickerKeep:
		m.theme = m.picker.current()
		m.themeChosen = true
		m.picker = nil
		if m.prefs != nil {
			if err := m.prefs.SetTheme(m.theme.Name); err != nil {
				m.notice = err.Error()
			}
		}
	case pickerCancel:
		m.cancelPicker()
	}
}

// cancelPicker closes the theme picker and goes back to the theme it
// started on.
func (m *Model) cancelPicker() {
	m.theme = m.picker.original
	m.picker = nil
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
	v.WindowTitle = "floe"
	v.Cursor = cursor
	return v
}
