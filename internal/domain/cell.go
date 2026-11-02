package domain

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

// Style describes how a cell is drawn.
type Style struct {
	Fg, Bg         Color
	UnderlineColor Color
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
	// Link is the cell's place in its grid's Links, counting from 1, or 0 for
	// no link. A link usually spans many cells and a URL can be long, so the
	// grid holds each one once.
	Link int
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
	// Links are the hyperlinks the cells refer to.
	Links []Link
}

// LinkOf returns the hyperlink on a cell of the grid, if it has one.
func (g Grid) LinkOf(c Cell) (Link, bool) {
	if c.Link < 1 || c.Link > len(g.Links) {
		return Link{}, false
	}
	return g.Links[c.Link-1], true
}

// At returns the cell at x, y. Out-of-range positions return a blank cell.
func (g Grid) At(x, y int) Cell {
	if x < 0 || y < 0 || x >= g.Size.W || y >= g.Size.H {
		return Cell{Content: " ", Width: 1}
	}
	return g.Cells[y*g.Size.W+x]
}
