// Package theme is the catalog of color themes the client can draw with.
// Themes are plain data using standard library colors, so this package has
// no dependencies and knows nothing about how they are drawn.
package theme

import (
	"fmt"
	"image/color"
	"slices"
)

// Theme is the set of colors the multiplexer's own chrome is drawn with:
// frames, titles, the status bar and popups. Panes keep their own colors.
type Theme struct {
	// Name identifies the theme in settings, such as "tokyo-night".
	Name string
	// Label is the name shown to people, such as "Tokyo Night".
	Label string
	// Dark is set for themes made for dark terminal backgrounds.
	Dark bool

	Accent      color.Color // focused frame, badges, selection
	Border      color.Color // unfocused frames
	Title       color.Color // unfocused titles
	BarBg       color.Color // status bar and panel background
	BarFg       color.Color // status bar and panel text
	BarDim      color.Color // hints and secondary text
	BadgeFg     color.Color // text on the accent color
	ActiveTabBg color.Color // focused tab in the status bar
	Error       color.Color
}

// Default is the theme used until one is chosen.
const Default = "violet"

// DefaultLight is the theme used until one is chosen, on a light terminal.
const DefaultLight = "catppuccin-latte"

var catalog = []Theme{
	{
		Name: "violet", Label: "Violet", Dark: true,
		Accent: hex("#8B5CF6"), Border: hex("#4B5563"), Title: hex("#9CA3AF"),
		BarBg: hex("#1F2937"), BarFg: hex("#E5E7EB"), BarDim: hex("#9CA3AF"),
		BadgeFg: hex("#FFFFFF"), ActiveTabBg: hex("#374151"), Error: hex("#F87171"),
	},
	{
		Name: "catppuccin-mocha", Label: "Catppuccin Mocha", Dark: true,
		Accent: hex("#CBA6F7"), Border: hex("#6C7086"), Title: hex("#A6ADC8"),
		BarBg: hex("#181825"), BarFg: hex("#CDD6F4"), BarDim: hex("#9399B2"),
		BadgeFg: hex("#11111B"), ActiveTabBg: hex("#313244"), Error: hex("#F38BA8"),
	},
	{
		Name: "catppuccin-latte", Label: "Catppuccin Latte", Dark: false,
		Accent: hex("#8839EF"), Border: hex("#9CA0B0"), Title: hex("#6C6F85"),
		BarBg: hex("#DCE0E8"), BarFg: hex("#4C4F69"), BarDim: hex("#5C5F77"),
		BadgeFg: hex("#EFF1F5"), ActiveTabBg: hex("#CCD0DA"), Error: hex("#D20F39"),
	},
	{
		Name: "tokyo-night", Label: "Tokyo Night", Dark: true,
		Accent: hex("#7AA2F7"), Border: hex("#565F89"), Title: hex("#A9B1D6"),
		BarBg: hex("#16161E"), BarFg: hex("#C0CAF5"), BarDim: hex("#737AA2"),
		BadgeFg: hex("#1A1B26"), ActiveTabBg: hex("#292E42"), Error: hex("#F7768E"),
	},
	{
		Name: "gruvbox", Label: "Gruvbox", Dark: true,
		Accent: hex("#FE8019"), Border: hex("#665C54"), Title: hex("#A89984"),
		BarBg: hex("#1D2021"), BarFg: hex("#EBDBB2"), BarDim: hex("#A89984"),
		BadgeFg: hex("#1D2021"), ActiveTabBg: hex("#3C3836"), Error: hex("#FB4934"),
	},
	{
		Name: "nord", Label: "Nord", Dark: true,
		Accent: hex("#88C0D0"), Border: hex("#4C566A"), Title: hex("#81A1C1"),
		BarBg: hex("#2E3440"), BarFg: hex("#ECEFF4"), BarDim: hex("#9AA5B8"),
		BadgeFg: hex("#2E3440"), ActiveTabBg: hex("#434C5E"), Error: hex("#BF616A"),
	},
	{
		Name: "rose-pine", Label: "Rosé Pine", Dark: true,
		Accent: hex("#EBBCBA"), Border: hex("#524F67"), Title: hex("#908CAA"),
		BarBg: hex("#1F1D2E"), BarFg: hex("#E0DEF4"), BarDim: hex("#908CAA"),
		BadgeFg: hex("#191724"), ActiveTabBg: hex("#403D52"), Error: hex("#EB6F92"),
	},
	{
		Name: "dracula", Label: "Dracula", Dark: true,
		Accent: hex("#BD93F9"), Border: hex("#6272A4"), Title: hex("#9AA5CE"),
		BarBg: hex("#21222C"), BarFg: hex("#F8F8F2"), BarDim: hex("#9AA5CE"),
		BadgeFg: hex("#21222C"), ActiveTabBg: hex("#44475A"), Error: hex("#FF5555"),
	},
}

// All returns every theme, in the order they are offered.
func All() []Theme {
	return slices.Clone(catalog)
}

// Lookup returns the theme with the given name.
func Lookup(name string) (Theme, bool) {
	i := slices.IndexFunc(catalog, func(t Theme) bool { return t.Name == name })
	if i < 0 {
		return Theme{}, false
	}
	return catalog[i], true
}

// MustLookup is Lookup for names known to be in the catalog, such as
// Default.
func MustLookup(name string) Theme {
	t, ok := Lookup(name)
	if !ok {
		panic(fmt.Sprintf("theme %q is not in the catalog", name))
	}
	return t
}

// hex parses a "#RRGGBB" color. The catalog is fixed, so a malformed value
// is a programming error.
func hex(s string) color.Color {
	var r, g, b uint8
	if _, err := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b); err != nil || len(s) != 7 {
		panic(fmt.Sprintf("bad color %q", s))
	}
	return color.RGBA{R: r, G: g, B: b, A: 0xFF}
}
