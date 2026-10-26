package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/input"
)

// render draws the whole screen and returns it with the cursor position.
func (m Model) render() (string, *tea.Cursor) {
	if m.width <= 0 || m.height <= 0 {
		return "", nil
	}
	canvas := lipgloss.NewCanvas(m.width, m.height)

	var cursor *tea.Cursor
	for _, w := range m.snap.Windows {
		m.drawFrame(canvas, w)
		grid, ok := m.backend.Grid(w.Pane)
		if !ok {
			continue
		}
		drawGrid(canvas, w.Content, grid)
		if w.Focused {
			cursor = gridCursor(w.Content, grid)
		}
	}

	if m.drag != nil && m.drag.zone != domain.ZoneNone {
		m.drawZone(canvas, m.drag.zone.Rect(m.snap.Area))
	}
	if m.switcher != nil {
		m.switcher.draw(canvas, m.snap.Area, m.sessions, m.theme)
		cursor = nil
	}
	if m.keys.Mode() == input.Prefix {
		m.drawWhichKey(canvas, m.snap.Area)
	}

	bar := uv.Rect(0, m.height-statusBarHeight, m.width, statusBarHeight)
	if m.prompt != nil {
		line, x := m.promptBar(m.width)
		uv.NewStyledString(line).Draw(canvas, bar)
		cursor = tea.NewCursor(x, bar.Min.Y)
	} else {
		uv.NewStyledString(m.statusBar(m.width)).Draw(canvas, bar)
	}

	return canvas.Render(), cursor
}

// border is the set of characters a frame is drawn with.
type border struct {
	topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical string
}

var (
	roundedBorder = border{"╭", "╮", "╰", "╯", "─", "│"}
	// heavyBorder marks the window being dragged.
	heavyBorder = border{"┏", "┓", "┗", "┛", "━", "┃"}
	// snapBorder outlines the zone a dragged window will snap to.
	snapBorder = border{"╔", "╗", "╚", "╝", "═", "║"}
)

func (m Model) drawFrame(scr uv.Screen, w domain.WindowView) {
	f := w.Frame
	if f.W < 2 || f.H < 2 {
		return
	}
	// Clear the whole frame so windows behind never show through.
	blank := uv.EmptyCell
	for y := f.Y; y < f.Bottom(); y++ {
		for x := f.X; x < f.Right(); x++ {
			scr.SetCell(x, y, &blank)
		}
	}

	style := uv.Style{Fg: m.theme.Border}
	if w.Focused {
		style.Fg = m.theme.Accent
	}
	b := roundedBorder
	if m.drag != nil && m.drag.window == w.ID {
		b = heavyBorder
	}
	set := func(x, y int, s string) {
		c := uv.Cell{Content: s, Width: 1, Style: style}
		scr.SetCell(x, y, &c)
	}

	right, bottom := f.Right()-1, f.Bottom()-1
	for x := f.X + 1; x < right; x++ {
		set(x, f.Y, b.horizontal)
		set(x, bottom, b.horizontal)
	}
	for y := f.Y + 1; y < bottom; y++ {
		set(f.X, y, b.vertical)
		set(right, y, b.vertical)
	}
	set(f.X, f.Y, b.topLeft)
	set(right, f.Y, b.topRight)
	set(f.X, bottom, b.bottomLeft)
	set(right, bottom, b.bottomRight)

	// The title gets the top border between the left corner and the
	// buttons, keeping one line cell on each side.
	titleEnd := right - 1
	if maxCol, closeCol, ok := buttons(f); ok {
		set(maxCol-1, f.Y, " ")
		set(maxCol, f.Y, "□")
		set(closeCol-1, f.Y, " ")
		set(closeCol, f.Y, "×")
		set(closeCol+1, f.Y, " ")
		titleEnd = maxCol - 2
	}
	room := titleEnd - (f.X + 2)
	if room < 3 {
		return
	}
	titleStyle := lipgloss.NewStyle().Foreground(m.theme.Title)
	if w.Focused {
		titleStyle = titleStyle.Foreground(m.theme.Accent).Bold(true)
	}
	title := ansi.Truncate(fmt.Sprintf(" %d %s ", w.ID, w.Title), room, "… ")
	area := uv.Rect(f.X+2, f.Y, lipgloss.Width(title), 1)
	uv.NewStyledString(titleStyle.Render(title)).Draw(scr, area)
}

// drawZone outlines where a dragged window will snap, over everything
// else.
func (m Model) drawZone(scr uv.Screen, r domain.Rect) {
	if r.W < 2 || r.H < 2 {
		return
	}
	b := snapBorder
	style := uv.Style{Fg: m.theme.Accent, Attrs: uv.AttrBold}
	set := func(x, y int, s string) {
		c := uv.Cell{Content: s, Width: 1, Style: style}
		scr.SetCell(x, y, &c)
	}
	right, bottom := r.Right()-1, r.Bottom()-1
	for x := r.X + 1; x < right; x++ {
		set(x, r.Y, b.horizontal)
		set(x, bottom, b.horizontal)
	}
	for y := r.Y + 1; y < bottom; y++ {
		set(r.X, y, b.vertical)
		set(right, y, b.vertical)
	}
	set(r.X, r.Y, b.topLeft)
	set(right, r.Y, b.topRight)
	set(r.X, bottom, b.bottomLeft)
	set(right, bottom, b.bottomRight)
}

// drawGrid copies a pane's screen into area. The grid may briefly differ in
// size from the area while a resize reaches the pane, so both are clipped.
func drawGrid(scr uv.Screen, area domain.Rect, g domain.Grid) {
	for y := range min(g.Size.H, area.H) {
		for x := range min(g.Size.W, area.W) {
			c := g.At(x, y)
			if c.Width == 0 || x+c.Width > area.W {
				continue
			}
			uc := toUVCell(c)
			if uc.Content == "" {
				uc.Content = " "
			}
			scr.SetCell(area.X+x, area.Y+y, &uc)
		}
	}
}

func gridCursor(area domain.Rect, g domain.Grid) *tea.Cursor {
	if !g.CursorVisible || g.Cursor.X >= area.W || g.Cursor.Y >= area.H {
		return nil
	}
	return tea.NewCursor(area.X+g.Cursor.X, area.Y+g.Cursor.Y)
}

func (m Model) statusBar(width int) string {
	t := m.theme
	bar := lipgloss.NewStyle().Background(t.BarBg).Foreground(t.BarFg)
	dim := bar.Foreground(t.BarDim)
	badge := m.badgeStyle()

	var left strings.Builder
	left.WriteString(badge.Render(m.sessionLabel()))
	left.WriteString(bar.Render(" "))
	for _, tab := range m.statusTabs() {
		if tab.window.Focused {
			left.WriteString(bar.Background(t.ActiveTabBg).Bold(true).Render(tab.label))
		} else {
			left.WriteString(dim.Render(tab.label))
		}
	}

	var right string
	switch {
	case m.notice != "":
		right = bar.Foreground(t.Error).Render("✗ " + m.notice + " ")
	case m.keys.Mode() == input.Prefix:
		// The full list is in the which-key popup.
		right = badge.Render("PREFIX") + dim.Render(" esc cancel ")
	case m.keys.Mode() == input.Repeat:
		right = badge.Render("REPEAT") + bar.Render(" ") + m.hints(bar, dim, true) + dim.Render("· any other key exits ")
	default:
		right = bar.Bold(true).Render(m.keys.Keymap().Prefix.String()) + dim.Render(" prefix ")
	}

	l := left.String()
	room := width - lipgloss.Width(l) - 1
	if lipgloss.Width(right) > room {
		right = ansi.Truncate(right, max(room, 0), "…")
	}
	gap := max(width-lipgloss.Width(l)-lipgloss.Width(right), 0)
	return ansi.Truncate(l+bar.Render(strings.Repeat(" ", gap))+right, width, "")
}

// statusTab is a window's tab in the status bar, covering columns
// [x0, x1).
type statusTab struct {
	window domain.WindowView
	label  string
	x0, x1 int
}

// statusTabs lays out the window tabs that follow the session badge, in
// window order. Drawing and mouse clicks both use it, so a click always
// lands on the tab drawn there.
func (m Model) statusTabs() []statusTab {
	windows := slices.Clone(m.snap.Windows)
	slices.SortFunc(windows, func(a, b domain.WindowView) int { return cmp.Compare(a.ID, b.ID) })

	x := m.badgeWidth() + 1
	tabs := make([]statusTab, len(windows))
	for i, w := range windows {
		label := fmt.Sprintf(" %d %s ", w.ID, w.Title)
		width := lipgloss.Width(label)
		tabs[i] = statusTab{window: w, label: label, x0: x, x1: x + width}
		x += width
	}
	return tabs
}

func (m Model) sessionLabel() string {
	if m.snap.Name == "" {
		return "mux"
	}
	return m.snap.Name
}

// badgeWidth is how many columns the session badge takes.
func (m Model) badgeWidth() int {
	return lipgloss.Width(m.badgeStyle().Render(m.sessionLabel()))
}

func (m Model) badgeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(m.theme.Accent).Foreground(m.theme.BadgeFg).Bold(true).Padding(0, 1)
}

// promptBar draws the open prompt in place of the status bar and returns
// the column where the cursor goes.
func (m Model) promptBar(width int) (string, int) {
	t := m.theme
	bar := lipgloss.NewStyle().Background(t.BarBg).Foreground(t.BarFg)
	badge := lipgloss.NewStyle().Background(t.Accent).Foreground(t.BadgeFg).Bold(true).Padding(0, 1)

	label := badge.Render(m.prompt.label) + bar.Render(" ")
	text := string(m.prompt.text)
	hint := bar.Foreground(t.BarDim).Render("enter save · esc cancel ")
	cursor := lipgloss.Width(label) + lipgloss.Width(text)

	gap := max(width-lipgloss.Width(label)-lipgloss.Width(text)-lipgloss.Width(hint), 1)
	line := label + bar.Render(text+strings.Repeat(" ", gap)) + hint
	return ansi.Truncate(line, width, ""), min(cursor, width-1)
}

// hint is one line of help: the keys for a command and what it does.
type hint struct {
	keys  []domain.Key
	help  string
	group string
}

// hintList describes the keymap compactly, folding the directional bindings
// of focus and resize, and the session cycling pair, into one entry each.
func (m Model) hintList(repeatOnly bool) []hint {
	var list []hint
	index := map[string]int{}
	for _, b := range m.keys.Keymap().Bindings {
		if repeatOnly && !b.Repeat {
			continue
		}
		name := b.Help
		switch b.Command.(type) {
		case domain.FocusDirection:
			name = "focus"
		case domain.ResizeWindow:
			name = "resize"
		}
		if b.Action == input.ActionPrevSession || b.Action == input.ActionNextSession {
			name = "prev/next"
		}
		if i, ok := index[name]; ok {
			list[i].keys = append(list[i].keys, b.Keys[0])
			continue
		}
		index[name] = len(list)
		list = append(list, hint{keys: []domain.Key{b.Keys[0]}, help: name, group: b.Group})
	}
	return list
}

// hints renders hintList on one line for the status bar.
func (m Model) hints(key, dim lipgloss.Style, repeatOnly bool) string {
	list := m.hintList(repeatOnly)
	parts := make([]string, len(list))
	for i, h := range list {
		parts[i] = key.Bold(true).Render(joinKeys(h.keys)) + dim.Render(" "+h.help)
	}
	return strings.Join(parts, dim.Render(" · ")) + dim.Render(" ")
}

// joinKeys writes single-character keys together, like "hjkl", and anything
// else separated by slashes.
func joinKeys(keys []domain.Key) string {
	names := make([]string, len(keys))
	short := true
	for i, k := range keys {
		names[i] = k.String()
		short = short && len([]rune(names[i])) == 1
	}
	if short {
		return strings.Join(names, "")
	}
	return strings.Join(names, "/")
}
