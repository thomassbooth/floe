package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme is the set of colors the UI draws with.
type Theme struct {
	Accent      color.Color // focused frame, badges
	Border      color.Color // unfocused frames
	Title       color.Color // unfocused titles
	BarBg       color.Color // status bar background
	BarFg       color.Color // status bar text
	BarDim      color.Color // status bar hints
	BadgeFg     color.Color
	ActiveTabBg color.Color
	Error       color.Color
}

// DefaultTheme is a violet accent on muted grays.
func DefaultTheme() Theme {
	return Theme{
		Accent:      lipgloss.Color("#8B5CF6"),
		Border:      lipgloss.Color("#4B5563"),
		Title:       lipgloss.Color("#9CA3AF"),
		BarBg:       lipgloss.Color("#1F2937"),
		BarFg:       lipgloss.Color("#E5E7EB"),
		BarDim:      lipgloss.Color("#9CA3AF"),
		BadgeFg:     lipgloss.Color("#FFFFFF"),
		ActiveTabBg: lipgloss.Color("#374151"),
		Error:       lipgloss.Color("#F87171"),
	}
}
