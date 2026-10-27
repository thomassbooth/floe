package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/theme"
)

// themePicker is the theme list overlay. Moving through it previews each
// theme on the whole screen.
type themePicker struct {
	themes   []theme.Theme
	selected int
	// original is the theme to go back to on cancel.
	original theme.Theme
}

func newThemePicker(current theme.Theme) *themePicker {
	p := &themePicker{themes: theme.All(), original: current}
	for i, t := range p.themes {
		if t.Name == current.Name {
			p.selected = i
		}
	}
	return p
}

// pickerResult is what a key does to the theme picker.
type pickerResult int

const (
	pickerOpen   pickerResult = iota // still choosing
	pickerKeep                       // keep the previewed theme
	pickerCancel                     // go back to the original theme
)

func (p *themePicker) handle(k domain.Key) pickerResult {
	switch {
	case k.Code == domain.KeyUp || k.Code == 'k':
		p.selected = max(p.selected-1, 0)
	case k.Code == domain.KeyDown || k.Code == 'j':
		p.selected = min(p.selected+1, len(p.themes)-1)
	case k.Code == domain.KeyEnter:
		return pickerKeep
	case k.Code == domain.KeyEscape || k.Code == 'q':
		return pickerCancel
	}
	return pickerOpen
}

// current is the theme being previewed.
func (p *themePicker) current() theme.Theme {
	return p.themes[p.selected]
}

const themePickerHelp = "↑↓ preview · enter keep · esc cancel"

// draw renders the picker centered in area, in the previewed theme. Each
// row ends with swatches of its theme's colors, drawn as colored cells so
// they are one column wide in every terminal.
func (p *themePicker) draw(scr uv.Screen, area domain.Rect, t theme.Theme) {
	labelW := 0
	for _, th := range p.themes {
		labelW = max(labelW, lipgloss.Width(th.Label))
	}
	const swatchW = 4 * 2 // four colors, two cells each
	inner := max(labelW+4+swatchW+1, lipgloss.Width(themePickerHelp)+2)
	inner = min(inner, area.W-4)
	if inner < 16 || area.H < 6 {
		return
	}

	panel := lipgloss.NewStyle().Background(t.BarBg).Foreground(t.BarFg)
	selected := lipgloss.NewStyle().Background(t.Accent).Foreground(t.BadgeFg).Bold(true)

	rows := []string{
		panel.Bold(true).Foreground(t.Accent).Width(inner).Render(" Themes"),
		panel.Width(inner).Render(""),
	}
	for i, th := range p.themes {
		marker := " "
		if th.Name == p.original.Name {
			marker = "●"
		}
		label := " " + marker + " " + th.Label
		label = ansi.Truncate(label, inner-swatchW-1, "…")
		gap := strings.Repeat(" ", max(inner-swatchW-1-lipgloss.Width(label), 0))
		style := panel
		if i == p.selected {
			style = selected
		}
		var swatches strings.Builder
		for _, c := range []color.Color{th.Accent, th.BarBg, th.BarFg, th.Error} {
			swatches.WriteString(lipgloss.NewStyle().Background(c).Render("  "))
		}
		rows = append(rows, style.Render(label+gap)+swatches.String()+style.Render(" "))
	}
	rows = append(rows,
		panel.Width(inner).Render(""),
		panel.Foreground(t.BarDim).Width(inner).Render(ansi.Truncate(" "+themePickerHelp, inner, "…")),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		BorderBackground(t.BarBg).
		Render(strings.Join(rows, "\n"))
	w, h := lipgloss.Width(box), min(lipgloss.Height(box), area.H)
	x := area.X + (area.W-w)/2
	y := area.Y + (area.H-h)/2
	uv.NewStyledString(box).Draw(scr, uv.Rect(x, y, w, h))
}
