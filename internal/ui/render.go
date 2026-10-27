package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/input"
)

// render draws the whole screen and returns it with the cursor position.
func (m Model) render() (string, *tea.Cursor) {
	if m.width <= 0 || m.height <= 0 {
		return "", nil
	}
	canvas := lipgloss.NewCanvas(m.width, m.height)

	var cursor *tea.Cursor
	for _, w := range m.snap.Windows {
		if w.Minimized {
			continue
		}
		w = m.drawnView(w)
		m.drawFrame(canvas, w)
		grid, ok := m.backend.Grid(w.Pane, m.scroll[w.Pane])
		if !ok {
			continue
		}
		drawGrid(canvas, w.Content, grid)
		m.drawSelection(canvas, w)
		m.drawScrollMarker(canvas, w)
		if w.Focused {
			cursor = gridCursor(w.Content, grid)
		}
	}
	if m.anim != nil {
		cursor = nil // it would jump around with the window
	}

	if m.arranging() {
		cursor = nil // keys go to arranging, not the shell
	}
	if m.allMinimized() {
		m.drawRestoreHint(canvas, m.snap.Area)
	}
	if m.drag != nil && m.drag.zone != domain.ZoneNone {
		m.drawZone(canvas, m.drag.zone.Rect(m.snap.Area))
	}
	if m.keys.Mode() == input.Prefix {
		m.drawWhichKey(canvas, m.snap.Area)
	}

	line, _ := m.statusBar(m.width)
	uv.NewStyledString(line).Draw(canvas, uv.Rect(0, m.statusBarRow(), m.width, statusBarHeight))

	if m.overlay != nil {
		if c, ok := m.overlay.draw(m, canvas); ok {
			cursor = c
		}
	}
	return canvas.Render(), cursor
}

// border is the set of characters a frame is drawn with.
type border struct {
	topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical string
}

var (
	roundedBorder = border{"╭", "╮", "╰", "╯", "─", "│"}
	// dashedBorder marks windows as movable in arrange mode.
	dashedBorder = border{"╭", "╮", "╰", "╯", "┄", "┆"}
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
	switch {
	case m.drag != nil && m.drag.window == w.ID:
		b = heavyBorder
	case m.arranging():
		b = dashedBorder
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
	if b, ok := buttons(f); ok {
		set(b.minimize-1, f.Y, " ")
		set(b.minimize, f.Y, "_")
		set(b.maximize-1, f.Y, " ")
		set(b.maximize, f.Y, "□")
		set(b.close-1, f.Y, " ")
		set(b.close, f.Y, "×")
		set(b.close+1, f.Y, " ")
		titleEnd = b.minimize - 2
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

// allMinimized reports whether the session has windows but shows none.
func (m Model) allMinimized() bool {
	for _, w := range m.snap.Windows {
		if !w.Minimized {
			return false
		}
	}
	return len(m.snap.Windows) > 0
}

// drawRestoreHint says how to bring windows back, in the middle of a
// screen left empty by minimizing them all.
func (m Model) drawRestoreHint(scr uv.Screen, area domain.Rect) {
	key := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(m.theme.Title)
	var restore string
	for _, b := range m.keys.Keymap().Bindings {
		if b.Name == "restore" && len(b.Keys) > 0 {
			restore = m.keys.Keymap().Prefix.String() + " " + b.Keys[0].String()
		}
	}
	line := dim.Render("Every window is minimized. Click a tab")
	if restore != "" {
		line += dim.Render(" or press ") + key.Render(restore)
	}
	line += dim.Render(" to bring one back.")
	width := lipgloss.Width(line)
	if width > area.W {
		return
	}
	at := uv.Rect(area.X+(area.W-width)/2, area.Y+area.H/2, width, 1)
	uv.NewStyledString(line).Draw(scr, at)
}

// drawScrollMarker shows how far up a scrolled window is, on its bottom
// border.
func (m Model) drawScrollMarker(scr uv.Screen, w domain.WindowView) {
	lines := m.scroll[w.Pane]
	if lines == 0 || w.Frame.H < 2 {
		return
	}
	label := fmt.Sprintf(" ↑ %d ", lines)
	width := lipgloss.Width(label)
	if w.Frame.W < width+4 {
		return
	}
	style := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)
	area := uv.Rect(w.Frame.Right()-2-width, w.Frame.Bottom()-1, width, 1)
	uv.NewStyledString(style.Render(label)).Draw(scr, area)
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
