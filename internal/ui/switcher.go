package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/theme"
)

// switcher is the session list overlay.
type switcher struct {
	selected int
}

func newSwitcher(list domain.SessionsChanged) *switcher {
	s := &switcher{}
	for i, info := range list.Sessions {
		if info.ID == list.Current {
			s.selected = i
		}
	}
	return s
}

// handle reacts to a key. It returns a command to send, if any, and whether
// the switcher should close.
func (s *switcher) handle(k domain.Key, sessions []domain.SessionInfo) (domain.Command, bool) {
	s.clamp(len(sessions))
	switch {
	case k.Code == domain.KeyUp || k.Code == 'k':
		s.selected = max(s.selected-1, 0)
	case k.Code == domain.KeyDown || k.Code == 'j':
		s.selected = min(s.selected+1, len(sessions)-1)
	case k.Code == domain.KeyEnter && len(sessions) > 0:
		return domain.SwitchSession{ID: sessions[s.selected].ID}, true
	case k.Code == 'n':
		return domain.CreateSession{}, true
	case k.Code == 'x' && len(sessions) > 0:
		return domain.KillSession{ID: sessions[s.selected].ID}, false
	case k.Code == domain.KeyEscape || k.Code == 'q':
		return nil, true
	}
	return nil, false
}

func (s *switcher) clamp(n int) {
	s.selected = min(max(s.selected, 0), max(n-1, 0))
}

const switcherHelp = "↑↓ select · enter switch · n new · x kill · esc close"

// draw renders the switcher centered in area.
func (s *switcher) draw(scr uv.Screen, area domain.Rect, list domain.SessionsChanged, t theme.Theme) {
	s.clamp(len(list.Sessions))

	nameWidth := 4
	for _, info := range list.Sessions {
		nameWidth = max(nameWidth, lipgloss.Width(info.Name))
	}
	inner := max(nameWidth+20, lipgloss.Width(switcherHelp)+2)
	inner = min(inner, area.W-4)
	if inner < 10 || area.H < 6 {
		return
	}

	panel := lipgloss.NewStyle().Background(t.BarBg).Foreground(t.BarFg).Width(inner)
	dim := panel.Foreground(t.BarDim)
	selected := lipgloss.NewStyle().Background(t.Accent).Foreground(t.BadgeFg).Bold(true).Width(inner)

	rows := []string{
		panel.Bold(true).Foreground(t.Accent).Render(" Sessions"),
		panel.Render(""),
	}
	for i, info := range list.Sessions {
		marker := " "
		if info.ID == list.Current {
			marker = "●"
		}
		windows := fmt.Sprintf("%d window", info.Windows)
		if info.Windows != 1 {
			windows += "s"
		}
		line := fmt.Sprintf(" %s %-*s  %s", marker, nameWidth, info.Name, windows)
		line = ansi.Truncate(line, inner, "…")
		if i == s.selected {
			rows = append(rows, selected.Render(line))
		} else {
			rows = append(rows, panel.Render(line))
		}
	}
	rows = append(rows, panel.Render(""), dim.Render(ansi.Truncate(" "+switcherHelp, inner, "…")))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		BorderBackground(t.BarBg).
		Render(strings.Join(rows, "\n"))

	w, h := lipgloss.Width(box), lipgloss.Height(box)
	h = min(h, area.H)
	x := area.X + (area.W-w)/2
	y := area.Y + (area.H-h)/2
	uv.NewStyledString(box).Draw(scr, uv.Rect(x, y, w, h))
}
