package ui

import (
	"fmt"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/input"
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

// capture sends a press, the drag after it and its release to the program
// in one window, even if the mouse leaves the window on the way.
type capture struct {
	window  domain.WindowID
	content domain.Rect
	button  domain.MouseButton
}

// drag is a move or resize in progress.
type drag struct {
	window domain.WindowID
	// move is set when dragging by the title bar; otherwise edges are being
	// dragged.
	move   bool
	offset domain.Point // mouse position within the frame while moving
	size   domain.Size  // frame size while moving
	edges  []domain.Direction
	// start is where the button went down. The drag only begins once the
	// mouse leaves that cell: some terminals report motion on the same cell
	// during a plain click, which must not move or snap anything.
	start domain.Point
	moved bool
	// zone is where the window snaps if dropped now.
	zone domain.Zone
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
	m.selection, m.info = nil, ""
	if m.switcher != nil {
		m.switcher = nil
		return
	}
	if m.picker != nil {
		m.cancelPicker()
		return
	}
	if mouse.Y == m.height-statusBarHeight {
		m.statusBarClick(mouse.X, mouse.Button)
		return
	}
	t, ok := m.targetAt(mouse.X, mouse.Y)
	if !ok {
		return
	}
	id := t.window.ID
	if t.part == partContent && m.wantsMouse(t.window) {
		m.keys.Reset()
		m.backend.Apply(domain.FocusWindow{ID: id})
		m.capture = &capture{window: id, content: t.window.Content, button: mouseButtons[mouse.Button]}
		m.forward(m.capture, domain.MousePress, mouse)
		return
	}
	if mouse.Button != tea.MouseLeft {
		return
	}
	if t.part == partContent && !m.arranging() {
		m.keys.Reset()
		m.backend.Apply(domain.FocusWindow{ID: id})
		m.startSelection(t.window, mouse.X, mouse.Y)
		return
	}
	m.keys.Reset()
	m.backend.Apply(domain.FocusWindow{ID: id})

	// Moving and resizing only happen in arrange mode.
	switch t.part {
	case partClose:
		m.backend.Apply(domain.CloseWindow{ID: id})
	case partMaximize:
		m.backend.Apply(domain.ToggleMaximize{ID: id})
	case partTitle:
		if m.arranging() {
			m.drag = m.moveDrag(t.window, mouse.X, mouse.Y)
		}
	case partEdge:
		if m.arranging() {
			m.drag = &drag{window: id, edges: t.edges, start: domain.Point{X: mouse.X, Y: mouse.Y}}
		}
	}
}

func (m Model) arranging() bool {
	return m.keys.Mode() == input.Arrange
}

// statusBarClick handles a click on the status bar. A left click on the
// session badge opens the session switcher and one on a window's tab
// focuses that window; a middle click on a tab closes the window.
func (m *Model) statusBarClick(x int, button tea.MouseButton) {
	if m.prompt != nil || x >= m.width {
		return // the prompt replaces the badge and tabs
	}
	if _, toggle := m.statusBar(m.width); button == tea.MouseLeft && toggle.ok && x >= toggle.x0 && x < toggle.x1 {
		m.keys.ToggleArrange()
		return
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

// moveDrag starts moving a window grabbed by its title at x, y. The window
// keeps its size, snapped or not. Only a window filling the whole screen,
// which could not move otherwise, comes away smaller: at the size it had
// before being maximized, or else half the screen. The grab point keeps its
// place along the title.
func (m Model) moveDrag(w domain.WindowView, x, y int) *drag {
	f := w.Frame
	size := f.Size()
	if area := m.snap.Area; f == area {
		size = domain.Size{W: area.W / 2, H: area.H / 2}
		if !w.Restore.Empty() {
			size = w.Restore.Size()
		}
	}
	offset := domain.Point{X: x - f.X, Y: y - f.Y}
	if size != f.Size() {
		offset.X = min(max(offset.X*size.W/max(f.W, 1), 1), max(size.W-2, 1))
	}
	return &drag{window: w.ID, move: true, offset: offset, size: size, start: domain.Point{X: x, Y: y}}
}

// mouseMotion turns a drag into absolute commands, so a fast drag that
// skips cells still lands exactly under the mouse.
func (m *Model) mouseMotion(msg tea.MouseMotionMsg) {
	mouse := msg.Mouse()
	if m.capture != nil {
		m.forward(m.capture, domain.MouseMove, mouse)
		return
	}
	if s := m.selection; s != nil && s.dragging {
		s.head = domain.Point{
			X: min(max(mouse.X-s.content.X, 0), s.content.W-1),
			Y: min(max(mouse.Y-s.content.Y, 0), s.content.H-1),
		}
		return
	}
	if m.drag == nil {
		return
	}
	d := m.drag
	if !d.moved && mouse.X == d.start.X && mouse.Y == d.start.Y {
		return
	}
	d.moved = true
	if d.move {
		d.zone = domain.ZoneAt(domain.Point{X: mouse.X, Y: mouse.Y}, m.snap.Area)
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

// mouseRelease ends whatever the press started: a program's mouse capture,
// a text selection, which is copied, or a window drag, which may snap.
func (m *Model) mouseRelease(msg tea.MouseReleaseMsg) tea.Cmd {
	if c := m.capture; c != nil {
		m.capture = nil
		m.forward(c, domain.MouseRelease, msg.Mouse())
		return nil
	}
	if s := m.selection; s != nil && s.dragging {
		return m.finishSelection()
	}
	d := m.drag
	m.drag = nil
	if d == nil || !d.move || !d.moved {
		return nil
	}
	mouse := msg.Mouse()
	if zone := domain.ZoneAt(domain.Point{X: mouse.X, Y: mouse.Y}, m.snap.Area); zone != domain.ZoneNone {
		m.backend.Apply(domain.SnapWindow{ID: d.window, Zone: zone})
	}
	return nil
}

// click remembers a click, to spot double and triple clicks.
type click struct {
	at    time.Time
	x, y  int
	count int
}

// startSelection begins selecting at x, y in a window: a plain click starts
// a drag, a double click takes the word there and a triple click the line.
func (m *Model) startSelection(w domain.WindowView, x, y int) {
	now := time.Now()
	count := 1
	if prev := m.lastClick; prev.x == x && prev.y == y && now.Sub(prev.at) < multiClickWindow {
		count = prev.count%3 + 1
	}
	m.lastClick = click{at: now, x: x, y: y, count: count}

	p := domain.Point{X: x - w.Content.X, Y: y - w.Content.Y}
	s := &selection{
		window:   w.ID,
		pane:     w.Pane,
		content:  w.Content,
		scroll:   m.scroll[w.Pane],
		anchor:   p,
		head:     p,
		dragging: true,
	}
	if count > 1 {
		if g, ok := m.backend.Grid(w.Pane, s.scroll); ok {
			s.whole = true
			if count == 2 {
				from, to := wordAt(g, p.X, p.Y)
				s.anchor.X, s.head.X = from, to
			} else {
				s.anchor.X, s.head.X = 0, lineEnd(g, p.Y)
			}
		}
	}
	m.selection = s
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
