package domain

import "image/color"

// Attr is a set of text attributes.
type Attr uint8

const (
	AttrBold Attr = 1 << iota
	AttrFaint
	AttrItalic
	AttrBlink
	AttrReverse
	AttrConceal
	AttrStrikethrough
)

// Underline is the shape of a cell's underline.
type Underline uint8

const (
	UnderlineNone Underline = iota
	UnderlineSingle
	UnderlineDouble
	UnderlineCurly
	UnderlineDotted
	UnderlineDashed
)

// Style describes how a cell is drawn. A nil color means the terminal
// default.
type Style struct {
	Fg, Bg         color.Color
	UnderlineColor color.Color
	Underline      Underline
	Attrs          Attr
}

// Cell is one cell of a terminal screen. A wide character occupies its own
// cell with Width 2 followed by a cell with Width 0 that should be skipped
// when drawing.
type Cell struct {
	Content string
	Width   int
	Style   Style
}

// PaneState is what a client needs to know about a pane to scroll it and
// route input to it.
type PaneState struct {
	// Scrollback is how many lines of history sit above the screen.
	Scrollback int
	// AltScreen is set while a full-screen program, such as an editor, is
	// using the alternate screen, which has no history.
	AltScreen bool
	// Mouse is which mouse events the pane's program wants.
	Mouse MouseReport
}

// Grid is a copy of a pane's visible screen.
type Grid struct {
	Size          Size
	Cells         []Cell // row-major, len == Size.W * Size.H
	Cursor        Point
	CursorVisible bool
}

// At returns the cell at x, y. Out-of-range positions return a blank cell.
func (g Grid) At(x, y int) Cell {
	if x < 0 || y < 0 || x >= g.Size.W || y >= g.Size.H {
		return Cell{Content: " ", Width: 1}
	}
	return g.Cells[y*g.Size.W+x]
}
