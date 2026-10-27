package domain

// ColorKind says how a Color is given.
type ColorKind uint8

const (
	// ColorDefault is the terminal's own foreground or background.
	ColorDefault ColorKind = iota
	// ColorBasic is one of the 16 ANSI colors, Value 0-15. Terminals draw
	// these from their own palette, so they follow the user's theme.
	ColorBasic
	// ColorIndexed is one of the 256 palette colors, Value 0-255.
	ColorIndexed
	// ColorRGB is an exact color, Value 0xRRGGBB.
	ColorRGB
)

// Color is a cell's color as a program asked for it. It is a plain value, so
// it can be compared and sent to a client as it is. The zero Color is the
// terminal default.
type Color struct {
	Kind  ColorKind
	Value uint32
}

// BasicColor returns ANSI color n, 0-15.
func BasicColor(n uint8) Color { return Color{Kind: ColorBasic, Value: uint32(n & 0x0f)} }

// IndexedColor returns palette color n.
func IndexedColor(n uint8) Color { return Color{Kind: ColorIndexed, Value: uint32(n)} }

// RGBColor returns an exact color.
func RGBColor(r, g, b uint8) Color {
	return Color{Kind: ColorRGB, Value: uint32(r)<<16 | uint32(g)<<8 | uint32(b)}
}

// RGB returns the parts of a ColorRGB color.
func (c Color) RGB() (r, g, b uint8) {
	return uint8(c.Value >> 16), uint8(c.Value >> 8), uint8(c.Value)
}
