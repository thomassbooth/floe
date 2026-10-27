package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/input"
)

// statusBar draws the bottom line and says where its arrange toggle is.
func (m Model) statusBar(width int) (line string, bounds toggleBounds) {
	t := m.theme
	bar := lipgloss.NewStyle().Background(t.BarBg).Foreground(t.BarFg)
	dim := bar.Foreground(t.BarDim)
	badge := m.badgeStyle()

	var left strings.Builder
	left.WriteString(badge.Render(m.sessionLabel()))
	left.WriteString(bar.Render(" "))
	for _, tab := range m.statusTabs() {
		style := dim
		switch {
		case tab.window.Focused:
			style = bar.Background(t.ActiveTabBg).Bold(true)
		case tab.window.Minimized:
			style = dim.Italic(true)
		}
		left.WriteString(style.Render(tab.label))
		if tab.marker != "" {
			left.WriteString(style.Render(" "))
		}
		switch tab.marker {
		case bellMarker:
			left.WriteString(style.Foreground(t.Error).Bold(true).Render(tab.marker))
		case busyMarker:
			// The accent can be faint on the focused tab's background, so
			// there the dot takes the label's color.
			dot := style.Foreground(t.Accent)
			if tab.window.Focused {
				dot = style
			}
			left.WriteString(dot.Render(tab.marker))
		}
		left.WriteString(style.Render(" "))
	}

	var right, toggle string
	switch {
	case m.notice != "":
		right = bar.Foreground(t.Error).Render("✗ " + m.notice + " ")
	case m.info != "":
		right = bar.Foreground(t.Accent).Render("✓ " + m.info + " ")
	case m.keys.Mode() == input.Prefix:
		// The full list is in the which-key popup.
		right = badge.Render("PREFIX") + dim.Render(" esc cancel ")
	case m.keys.Mode() == input.Repeat:
		right = badge.Render("REPEAT") + bar.Render(" ") + hints(m.keys.Keymap().Bindings, bar, dim, true) + dim.Render("· any other key exits ")
	case m.keys.Mode() == input.Arrange:
		// The badge doubles as the button that leaves arrange mode.
		toggle = badge.Render("ARRANGE")
		right = toggle + bar.Render(" ") + hints(m.keys.Keymap().Arrange, bar, dim, false)
	default:
		toggle = bar.Background(t.ActiveTabBg).Render(" arrange ")
		right = toggle + bar.Render("  ") + bar.Bold(true).Render(m.keys.Keymap().Prefix.String()) + dim.Render(" prefix ")
	}

	l := left.String()
	room := width - lipgloss.Width(l) - 1
	if lipgloss.Width(right) > room {
		right = ansi.Truncate(right, max(room, 0), "…")
	}
	gap := max(width-lipgloss.Width(l)-lipgloss.Width(right), 0)
	if toggle != "" {
		x0 := lipgloss.Width(l) + gap
		bounds = toggleBounds{x0: x0, x1: min(x0+lipgloss.Width(toggle), width), ok: x0 < width}
	}
	return ansi.Truncate(l+bar.Render(strings.Repeat(" ", gap))+right, width, ""), bounds
}

// toggleBounds is where the arrange toggle sits in the status bar, columns
// [x0, x1).
type toggleBounds struct {
	x0, x1 int
	ok     bool
}

// statusTab is a window's tab in the status bar, covering columns
// [x0, x1): its label, a marker for a busy pane or a bell if any, and a
// space.
type statusTab struct {
	window domain.WindowView
	label  string
	marker string
	x0, x1 int
}

// Markers after a tab's label for a window whose program is busy printing,
// or that rang the bell while another had focus. The bell wins, since it
// asks for attention.
const (
	busyMarker = "•"
	bellMarker = "!"
)

// statusTabs lays out the window tabs that follow the session badge, in
// window order. Drawing and mouse clicks both use it, so a click always
// lands on the tab drawn there.
func (m Model) statusTabs() []statusTab {
	windows := slices.Clone(m.snap.Windows)
	slices.SortFunc(windows, func(a, b domain.WindowView) int { return cmp.Compare(a.ID, b.ID) })

	x := m.badgeWidth() + 1
	tabs := make([]statusTab, len(windows))
	for i, w := range windows {
		label := fmt.Sprintf(" %d %s", w.ID, w.Title)
		marker := ""
		switch {
		case w.Bell:
			marker = bellMarker
		case w.Busy:
			marker = busyMarker
		}
		width := lipgloss.Width(label) + 1 // and the space after
		if marker != "" {
			width += 1 + lipgloss.Width(marker) // a space, then the marker
		}
		tabs[i] = statusTab{window: w, label: label, marker: marker, x0: x, x1: x + width}
		x += width
	}
	return tabs
}

func (m Model) sessionLabel() string {
	if m.snap.Name == "" {
		return "floe"
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

// hint is one line of help: the keys for a command and what it does.
type hint struct {
	keys  []domain.Key
	help  string
	group string
}

// hintList describes bindings compactly, folding bindings with the same
// purpose, like the four directions of focus, into one entry each.
func hintList(bindings []input.Binding, repeatOnly bool) []hint {
	var list []hint
	index := map[string]int{}
	for _, b := range bindings {
		if len(b.Keys) == 0 || repeatOnly && !b.Repeat {
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
func hints(bindings []input.Binding, key, dim lipgloss.Style, repeatOnly bool) string {
	list := hintList(bindings, repeatOnly)
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
