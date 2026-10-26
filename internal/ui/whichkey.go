package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// columnGap separates the groups of the which-key popup.
const columnGap = 4

// whichKeyGroup is one section of the popup: a heading and its entries.
type whichKeyGroup struct {
	name    string
	entries []hint
	keyW    int // width of the widest key column
	width   int // width of the whole section
}

// drawWhichKey shows every binding after the prefix key, grouped into
// sections, in a panel at the bottom right of area. Sections sit side by
// side when they fit and stack otherwise.
func (m Model) drawWhichKey(scr uv.Screen, area domain.Rect) {
	// Stay off the outermost cells, where window frames usually are, and
	// leave a gap on the right.
	area = area.Inset(1)
	area.W--
	t := m.theme
	panel := lipgloss.NewStyle().Background(t.BarBg).Foreground(t.BarFg)
	heading := panel.Foreground(t.Accent).Bold(true)
	key := panel.Bold(true)
	dim := panel.Foreground(t.BarDim)

	groups := m.whichKeyGroups()
	if len(groups) == 0 {
		return
	}
	// The footer drops its first tip when space is tight.
	footer := m.keys.Keymap().Prefix.String() + " again sends it · esc cancel"
	if lipgloss.Width(footer)+4 > area.W {
		footer = "esc cancel"
	}

	// Side by side if that fits inside the area with the border and
	// padding, one section under another if not.
	sideBySide := -columnGap
	for _, g := range groups {
		sideBySide += g.width + columnGap
	}
	inner := max(sideBySide, lipgloss.Width(footer))
	columns := inner+4 <= area.W // border and padding on each side
	if !columns {
		// Stacked sections share one key column so their text lines up.
		keyW := 0
		for _, g := range groups {
			keyW = max(keyW, g.keyW)
		}
		inner = lipgloss.Width(footer)
		for i := range groups {
			g := &groups[i]
			g.width += keyW - g.keyW
			g.keyW = keyW
			inner = max(inner, g.width)
		}
	}

	pad := func(s string, w int) string { return s + strings.Repeat(" ", max(w-lipgloss.Width(s), 0)) }
	cell := func(g whichKeyGroup, row int) string {
		if row == 0 {
			return heading.Render(pad(g.name, g.width))
		}
		if row-1 >= len(g.entries) {
			return panel.Render(strings.Repeat(" ", g.width))
		}
		e := g.entries[row-1]
		return key.Render(pad(joinKeys(e.keys), g.keyW)) + panel.Render("  ") + panel.Render(pad(e.help, g.width-g.keyW-2))
	}

	var rows []string
	if columns {
		height := 0
		for _, g := range groups {
			height = max(height, len(g.entries)+1)
		}
		for r := range height {
			var line strings.Builder
			for i, g := range groups {
				if i > 0 {
					line.WriteString(panel.Render(strings.Repeat(" ", columnGap)))
				}
				line.WriteString(cell(g, r))
			}
			rows = append(rows, line.String())
		}
	} else {
		for i, g := range groups {
			if i > 0 {
				rows = append(rows, "")
			}
			for r := range len(g.entries) + 1 {
				rows = append(rows, cell(g, r))
			}
		}
	}
	rows = append(rows, "", dim.Render(footer))

	for i, r := range rows {
		rows[i] = panel.Render(" ") + r + panel.Render(strings.Repeat(" ", max(inner-lipgloss.Width(r), 0))+" ")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		BorderBackground(t.BarBg).
		Render(strings.Join(rows, "\n"))

	w, h := lipgloss.Width(box), lipgloss.Height(box)
	x := max(area.Right()-w-1, area.X)
	y := max(area.Bottom()-h, area.Y)
	uv.NewStyledString(box).Draw(scr, uv.Rect(x, y, min(w, area.W), min(h, area.H)))
}

// whichKeyGroups sorts the keymap's help into sections, in keymap order.
func (m Model) whichKeyGroups() []whichKeyGroup {
	var groups []whichKeyGroup
	index := map[string]int{}
	for _, h := range m.hintList(false) {
		i, ok := index[h.group]
		if !ok {
			i = len(groups)
			index[h.group] = i
			groups = append(groups, whichKeyGroup{name: h.group})
		}
		groups[i].entries = append(groups[i].entries, h)
	}
	for i := range groups {
		g := &groups[i]
		helpW := 0
		for _, e := range g.entries {
			g.keyW = max(g.keyW, lipgloss.Width(joinKeys(e.keys)))
			helpW = max(helpW, lipgloss.Width(e.help))
		}
		g.width = max(lipgloss.Width(g.name), g.keyW+2+helpW)
	}
	return groups
}
