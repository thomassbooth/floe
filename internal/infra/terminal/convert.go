package terminal

import (
	"image/color"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"github.com/thomassbooth/floe/internal/domain"
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
	return uv.KeyPressEvent{Code: code, Text: k.Text, Mod: toVTMod(k.Mod)}
}

var mouseButtons = map[domain.MouseButton]uv.MouseButton{
	domain.ButtonNone:   uv.MouseNone,
	domain.ButtonLeft:   uv.MouseLeft,
	domain.ButtonMiddle: uv.MouseMiddle,
	domain.ButtonRight:  uv.MouseRight,
	domain.WheelUp:      uv.MouseWheelUp,
	domain.WheelDown:    uv.MouseWheelDown,
}

func toVTMouse(ev domain.MouseEvent) uv.MouseEvent {
	m := uv.Mouse{X: ev.X, Y: ev.Y, Button: mouseButtons[ev.Button], Mod: toVTMod(ev.Mod)}
	switch ev.Action {
	case domain.MouseRelease:
		return uv.MouseReleaseEvent(m)
	case domain.MouseMove:
		return uv.MouseMotionEvent(m)
	case domain.MouseWheel:
		return uv.MouseWheelEvent(m)
	}
	return uv.MouseClickEvent(m)
}

func toVTMod(km domain.KeyMod) uv.KeyMod {
	var mod uv.KeyMod
	if km&domain.ModShift != 0 {
		mod |= uv.ModShift
	}
	if km&domain.ModAlt != 0 {
		mod |= uv.ModAlt
	}
	if km&domain.ModCtrl != 0 {
		mod |= uv.ModCtrl
	}
	if km&domain.ModSuper != 0 {
		mod |= uv.ModSuper
	}
	return mod
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
			Fg:             fromColor(c.Style.Fg),
			Bg:             fromColor(c.Style.Bg),
			UnderlineColor: fromColor(c.Style.UnderlineColor),
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

// fromColor keeps palette colors as palette colors, so the terminal floe runs
// in still draws them from its own theme.
func fromColor(c color.Color) domain.Color {
	switch c := c.(type) {
	case nil:
		return domain.Color{}
	case ansi.BasicColor:
		return domain.BasicColor(uint8(c))
	case ansi.IndexedColor:
		return domain.IndexedColor(uint8(c))
	}
	r, g, b, a := c.RGBA()
	if a == 0 {
		return domain.Color{}
	}
	// RGBA is premultiplied; undo that for the rare translucent color.
	r, g, b = r*0xffff/a, g*0xffff/a, b*0xffff/a
	return domain.RGBColor(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}
