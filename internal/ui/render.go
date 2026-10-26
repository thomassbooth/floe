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

	bar := uv.Rect(0, m.height-statusBarHeight, m.width, statusBarHeight)
	uv.NewStyledString(m.statusBar(m.width)).Draw(canvas, bar)

	return canvas.Render(), cursor
}

func (m Model) drawFrame(scr uv.Screen, w domain.WindowView) {
	f := w.Frame
	if f.W < 2 || f.H < 2 {
		return
	}
	style := uv.Style{Fg: m.theme.Border}
	if w.Focused {
		style.Fg = m.theme.Accent
	}
	set := func(x, y int, s string) {
		c := uv.Cell{Content: s, Width: 1, Style: style}
		scr.SetCell(x, y, &c)
	}

	right, bottom := f.Right()-1, f.Bottom()-1
	for x := f.X + 1; x < right; x++ {
		set(x, f.Y, "─")
		set(x, bottom, "─")
	}
	for y := f.Y + 1; y < bottom; y++ {
		set(f.X, y, "│")
		set(right, y, "│")
	}
	set(f.X, f.Y, "╭")
	set(right, f.Y, "╮")
	set(f.X, bottom, "╰")
	set(right, bottom, "╯")

	// Leave a corner and one line cell on each side of the title.
	room := f.W - 4
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
	badge := lipgloss.NewStyle().Background(t.Accent).Foreground(t.BadgeFg).Bold(true).Padding(0, 1)

	var left strings.Builder
	left.WriteString(badge.Render("mux"))
	left.WriteString(bar.Render(" "))
	windows := slices.Clone(m.snap.Windows)
	slices.SortFunc(windows, func(a, b domain.WindowView) int { return cmp.Compare(a.ID, b.ID) })
	for _, w := range windows {
		tab := fmt.Sprintf(" %d %s ", w.ID, w.Title)
		if w.Focused {
			left.WriteString(bar.Background(t.ActiveTabBg).Bold(true).Render(tab))
		} else {
			left.WriteString(dim.Render(tab))
		}
	}

	var right string
	switch {
	case m.notice != "":
		right = bar.Foreground(t.Error).Render("✗ " + m.notice + " ")
	case m.keys.Mode() == input.Prefix:
		right = badge.Render("PREFIX") + bar.Render(" ") + m.hints(bar, dim, false)
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

type hint struct {
	keys []domain.Key
	help string
}

// hints describes the keymap compactly, folding the four directional
// bindings of focus and resize into one entry each.
func (m Model) hints(key, dim lipgloss.Style, repeatOnly bool) string {
	var list []hint
	group := map[string]int{}
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
		if i, ok := group[name]; ok {
			list[i].keys = append(list[i].keys, b.Keys[0])
			continue
		}
		group[name] = len(list)
		list = append(list, hint{keys: []domain.Key{b.Keys[0]}, help: name})
	}

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
