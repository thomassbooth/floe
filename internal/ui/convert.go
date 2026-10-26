package ui

import (
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

var specialKeys = map[rune]rune{
	tea.KeyUp:     domain.KeyUp,
	tea.KeyDown:   domain.KeyDown,
	tea.KeyLeft:   domain.KeyLeft,
	tea.KeyRight:  domain.KeyRight,
	tea.KeyHome:   domain.KeyHome,
	tea.KeyEnd:    domain.KeyEnd,
	tea.KeyPgUp:   domain.KeyPgUp,
	tea.KeyPgDown: domain.KeyPgDown,
	tea.KeyInsert: domain.KeyInsert,
	tea.KeyDelete: domain.KeyDelete,
	tea.KeyF1:     domain.KeyF1,
	tea.KeyF2:     domain.KeyF2,
	tea.KeyF3:     domain.KeyF3,
	tea.KeyF4:     domain.KeyF4,
	tea.KeyF5:     domain.KeyF5,
	tea.KeyF6:     domain.KeyF6,
	tea.KeyF7:     domain.KeyF7,
	tea.KeyF8:     domain.KeyF8,
	tea.KeyF9:     domain.KeyF9,
	tea.KeyF10:    domain.KeyF10,
	tea.KeyF11:    domain.KeyF11,
	tea.KeyF12:    domain.KeyF12,
}

// toDomainKey translates a Bubble Tea key press. It reports false for keys
// the multiplexer does not support yet, such as keypad keys.
func toDomainKey(k tea.Key) (domain.Key, bool) {
	var mod domain.KeyMod
	if k.Mod.Contains(tea.ModShift) {
		mod |= domain.ModShift
	}
	if k.Mod.Contains(tea.ModAlt) {
		mod |= domain.ModAlt
	}
	if k.Mod.Contains(tea.ModCtrl) {
		mod |= domain.ModCtrl
	}
	if k.Mod.Contains(tea.ModSuper) {
		mod |= domain.ModSuper
	}

	if special, ok := specialKeys[k.Code]; ok {
		return domain.Key{Code: special, Mod: mod}, true
	}
	if k.Text != "" && mod&domain.ModCtrl == 0 {
		r, _ := utf8.DecodeRuneInString(k.Text)
		return domain.Key{Code: r, Text: k.Text, Mod: mod &^ domain.ModShift}, true
	}
	if k.Code > unicode.MaxRune {
		return domain.Key{}, false
	}
	return domain.Key{Code: k.Code, Mod: mod}, true
}

var attrs = []struct {
	from domain.Attr
	to   uint8
}{
	{domain.AttrBold, uv.AttrBold},
	{domain.AttrFaint, uv.AttrFaint},
	{domain.AttrItalic, uv.AttrItalic},
	{domain.AttrBlink, uv.AttrBlink},
	{domain.AttrReverse, uv.AttrReverse},
	{domain.AttrConceal, uv.AttrConceal},
	{domain.AttrStrikethrough, uv.AttrStrikethrough},
}

var underlines = map[domain.Underline]uv.Underline{
	domain.UnderlineSingle: uv.UnderlineSingle,
	domain.UnderlineDouble: uv.UnderlineDouble,
	domain.UnderlineCurly:  uv.UnderlineCurly,
	domain.UnderlineDotted: uv.UnderlineDotted,
	domain.UnderlineDashed: uv.UnderlineDashed,
}

func toUVCell(c domain.Cell) uv.Cell {
	out := uv.Cell{
		Content: c.Content,
		Width:   c.Width,
		Style: uv.Style{
			Fg:             c.Style.Fg,
			Bg:             c.Style.Bg,
			UnderlineColor: c.Style.UnderlineColor,
			Underline:      underlines[c.Style.Underline],
		},
	}
	for _, a := range attrs {
		if c.Style.Attrs&a.from != 0 {
			out.Style.Attrs |= a.to
		}
	}
	return out
}
