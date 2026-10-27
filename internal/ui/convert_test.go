package ui

import (
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestToDomainKey(t *testing.T) {
	tests := []struct {
		name string
		in   tea.Key
		want domain.Key
		ok   bool
	}{
		{"plain letter", tea.Key{Code: 'a', Text: "a"}, domain.Key{Code: 'a', Text: "a"}, true},
		{"shifted letter drops shift", tea.Key{Code: 'h', ShiftedCode: 'H', Text: "H", Mod: tea.ModShift}, domain.Key{Code: 'H', Text: "H"}, true},
		{"symbol", tea.Key{Code: '\\', Text: "|", Mod: tea.ModShift}, domain.Key{Code: '|', Text: "|"}, true},
		{"ctrl letter", tea.Key{Code: 'b', Mod: tea.ModCtrl}, domain.Key{Code: 'b', Mod: domain.ModCtrl}, true},
		{"alt letter keeps text", tea.Key{Code: 'x', Text: "x", Mod: tea.ModAlt}, domain.Key{Code: 'x', Text: "x", Mod: domain.ModAlt}, true},
		{"enter", tea.Key{Code: tea.KeyEnter}, domain.Key{Code: domain.KeyEnter}, true},
		{"shift arrow", tea.Key{Code: tea.KeyLeft, Mod: tea.ModShift}, domain.Key{Code: domain.KeyLeft, Mod: domain.ModShift}, true},
		{"function key", tea.Key{Code: tea.KeyF5}, domain.Key{Code: domain.KeyF5}, true},
		{"unsupported keypad key", tea.Key{Code: tea.KeyKpBegin}, domain.Key{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toDomainKey(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("got %#v, %v; want %#v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestToColorKeepsPaletteColors(t *testing.T) {
	tests := []struct {
		in   domain.Color
		want color.Color
	}{
		{domain.Color{}, nil},
		{domain.BasicColor(9), ansi.BasicColor(9)},
		{domain.IndexedColor(200), ansi.IndexedColor(200)},
		{domain.RGBColor(10, 20, 30), color.RGBA{R: 10, G: 20, B: 30, A: 0xff}},
	}
	for _, tt := range tests {
		if got := toColor(tt.in); got != tt.want {
			t.Errorf("toColor(%+v) = %#v, want %#v", tt.in, got, tt.want)
		}
	}
}
