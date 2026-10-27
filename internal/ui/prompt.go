package ui

import (
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/thomassbooth/floe/internal/domain"
)

// prompt is a one-line text input shown in place of the status bar. It
// stays open until its command succeeds, showing why if it fails, so a
// refused name can be fixed rather than typed again.
type prompt struct {
	label  string
	text   []rune
	submit func(string) domain.Command
	// err is why the last submit failed, until the text changes.
	err string
}

func newPrompt(label, initial string, submit func(string) domain.Command) *prompt {
	return &prompt{label: label, text: []rune(initial), submit: submit}
}

func (p *prompt) key(_ *Model, k domain.Key) (domain.Command, bool) {
	switch {
	case k.Code == domain.KeyEnter:
		cmd := p.submit(string(p.text))
		return cmd, cmd == nil
	case k.Code == domain.KeyEscape:
		return nil, true
	}
	before := string(p.text)
	switch {
	case k.Code == domain.KeyBackspace:
		if len(p.text) > 0 {
			p.text = p.text[:len(p.text)-1]
		}
	case k.Code == 'u' && k.Mod == domain.ModCtrl:
		p.text = nil
	case k.Text != "" && k.Mod&domain.ModCtrl == 0:
		for _, r := range k.Text {
			if unicode.IsPrint(r) {
				p.text = append(p.text, r)
			}
		}
	}
	if string(p.text) != before {
		p.err = ""
	}
	return nil, false
}

// result closes the prompt once its command went through, and otherwise
// keeps it open with the reason.
func (p *prompt) result(_ *Model, r domain.Result) bool {
	if r.Err != nil {
		p.err = r.Err.Error()
		return false
	}
	return true
}

// press leaves the prompt open, so windows can still be clicked, but
// swallows clicks on the status bar, which the prompt covers.
func (p *prompt) press(m *Model, at domain.Point) (bool, bool) {
	return false, at.Y != m.statusBarRow()
}

// draw puts the prompt over the status bar, with the cursor after the text.
func (p *prompt) draw(m Model, scr uv.Screen) (*tea.Cursor, bool) {
	t := m.theme
	width := m.width
	bar := lipgloss.NewStyle().Background(t.BarBg).Foreground(t.BarFg)
	badge := lipgloss.NewStyle().Background(t.Accent).Foreground(t.BadgeFg).Bold(true).Padding(0, 1)

	label := badge.Render(p.label) + bar.Render(" ")
	text := string(p.text)
	hint := bar.Foreground(t.BarDim).Render("enter save · esc cancel ")
	if p.err != "" {
		hint = bar.Foreground(t.Error).Render("✗ " + p.err + " ")
	}
	cursor := lipgloss.Width(label) + lipgloss.Width(text)

	gap := max(width-lipgloss.Width(label)-lipgloss.Width(text)-lipgloss.Width(hint), 1)
	line := ansi.Truncate(label+bar.Render(text+strings.Repeat(" ", gap))+hint, width, "")
	row := m.statusBarRow()
	uv.NewStyledString(line).Draw(scr, uv.Rect(0, row, width, statusBarHeight))
	return tea.NewCursor(min(cursor, width-1), row), true
}
