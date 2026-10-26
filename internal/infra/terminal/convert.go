package terminal

import (
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

var specialKeys = map[rune]rune{
	domain.KeyUp:     vt.KeyUp,
	domain.KeyDown:   vt.KeyDown,
	domain.KeyLeft:   vt.KeyLeft,
	domain.KeyRight:  vt.KeyRight,
	domain.KeyHome:   vt.KeyHome,
	domain.KeyEnd:    vt.KeyEnd,
	domain.KeyPgUp:   vt.KeyPgUp,
	domain.KeyPgDown: vt.KeyPgDown,
	domain.KeyInsert: vt.KeyInsert,
	domain.KeyDelete: vt.KeyDelete,
	domain.KeyF1:     vt.KeyF1,
	domain.KeyF2:     vt.KeyF2,
	domain.KeyF3:     vt.KeyF3,
	domain.KeyF4:     vt.KeyF4,
	domain.KeyF5:     vt.KeyF5,
	domain.KeyF6:     vt.KeyF6,
	domain.KeyF7:     vt.KeyF7,
	domain.KeyF8:     vt.KeyF8,
	domain.KeyF9:     vt.KeyF9,
	domain.KeyF10:    vt.KeyF10,
	domain.KeyF11:    vt.KeyF11,
	domain.KeyF12:    vt.KeyF12,
}

func toVTKey(k domain.Key) uv.KeyPressEvent {
	code := k.Code
	if special, ok := specialKeys[code]; ok {
		code = special
	}
	var mod uv.KeyMod
	if k.Mod&domain.ModShift != 0 {
		mod |= uv.ModShift
	}
	if k.Mod&domain.ModAlt != 0 {
		mod |= uv.ModAlt
	}
	if k.Mod&domain.ModCtrl != 0 {
		mod |= uv.ModCtrl
	}
	if k.Mod&domain.ModSuper != 0 {
		mod |= uv.ModSuper
	}
	return uv.KeyPressEvent{Code: code, Text: k.Text, Mod: mod}
}

var attrs = []struct {
	from uint8
	to   domain.Attr
}{
	{uv.AttrBold, domain.AttrBold},
	{uv.AttrFaint, domain.AttrFaint},
	{uv.AttrItalic, domain.AttrItalic},
	{uv.AttrBlink, domain.AttrBlink},
	{uv.AttrRapidBlink, domain.AttrBlink},
	{uv.AttrReverse, domain.AttrReverse},
	{uv.AttrConceal, domain.AttrConceal},
	{uv.AttrStrikethrough, domain.AttrStrikethrough},
}

var underlines = map[uv.Underline]domain.Underline{
	uv.UnderlineSingle: domain.UnderlineSingle,
	uv.UnderlineDouble: domain.UnderlineDouble,
	uv.UnderlineCurly:  domain.UnderlineCurly,
	uv.UnderlineDotted: domain.UnderlineDotted,
	uv.UnderlineDashed: domain.UnderlineDashed,
}

func fromVTCell(c *uv.Cell) domain.Cell {
	if c == nil {
		return domain.Cell{Content: " ", Width: 1}
	}
	out := domain.Cell{
		Content: c.Content,
		Width:   c.Width,
		Style: domain.Style{
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
