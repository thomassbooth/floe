package wire

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// Screens are most of the traffic, and gob spends many bytes on each cell,
// so a GridReply is encoded by hand. Cells are written in order; a cell's
// style is only written when it differs from the cell before, which in
// terminal text it rarely does.

// Cell flags.
const (
	cellStyle = 1 << iota // a style follows
	cellBlank             // the content is a single space
	cellWidth             // the width is not 1 and follows
)

// maxGridCells bounds a decoded grid, so a corrupt message cannot make the
// reader allocate without limit.
const maxGridCells = 1 << 22

// GobEncode writes the reply in the compact form.
func (r GridReply) GobEncode() ([]byte, error) {
	g := r.Grid
	if len(g.Cells) != g.Size.W*g.Size.H {
		return nil, fmt.Errorf("grid of %dx%d has %d cells", g.Size.W, g.Size.H, len(g.Cells))
	}
	b := make([]byte, 0, 64+len(g.Cells)*2)
	b = binary.AppendUvarint(b, r.ID)
	b = appendBool(b, r.OK)
	b = appendBool(b, g.CursorVisible)
	for _, n := range []int{g.Size.W, g.Size.H, g.Cursor.X, g.Cursor.Y} {
		b = binary.AppendVarint(b, int64(n))
	}
	var style domain.Style
	for i, c := range g.Cells {
		var flags byte
		if i == 0 || c.Style != style {
			flags |= cellStyle
		}
		if c.Content == " " {
			flags |= cellBlank
		}
		if c.Width != 1 {
			flags |= cellWidth
		}
		b = append(b, flags)
		if flags&cellWidth != 0 {
			b = binary.AppendVarint(b, int64(c.Width))
		}
		if flags&cellBlank == 0 {
			b = binary.AppendUvarint(b, uint64(len(c.Content)))
			b = append(b, c.Content...)
		}
		if flags&cellStyle != 0 {
			b = appendStyle(b, c.Style)
			style = c.Style
		}
	}
	return b, nil
}

// GobDecode reads a reply written by GobEncode.
func (r *GridReply) GobDecode(data []byte) error {
	d := decoder{b: data}
	r.ID = d.uvarint()
	r.OK = d.bool()
	g := domain.Grid{CursorVisible: d.bool()}
	g.Size.W, g.Size.H = d.int(), d.int()
	g.Cursor.X, g.Cursor.Y = d.int(), d.int()
	if d.err != nil {
		return d.err
	}
	if g.Size.W < 0 || g.Size.H < 0 || g.Size.W*g.Size.H > maxGridCells {
		return fmt.Errorf("bad grid size %dx%d", g.Size.W, g.Size.H)
	}
	g.Cells = make([]domain.Cell, g.Size.W*g.Size.H)
	var style domain.Style
	for i := range g.Cells {
		c := &g.Cells[i]
		flags := d.byte()
		c.Width = 1
		if flags&cellWidth != 0 {
			c.Width = d.int()
		}
		if flags&cellBlank != 0 {
			c.Content = " "
		} else {
			c.Content = d.string()
		}
		if flags&cellStyle != 0 {
			style = d.style()
		}
		c.Style = style
		if d.err != nil {
			return d.err
		}
	}
	if len(d.b) != 0 {
		return errors.New("grid has trailing bytes")
	}
	if len(g.Cells) == 0 {
		g.Cells = nil
	}
	r.Grid = g
	return nil
}

func appendBool(b []byte, v bool) []byte {
	if v {
		return append(b, 1)
	}
	return append(b, 0)
}

func appendStyle(b []byte, s domain.Style) []byte {
	for _, c := range []domain.Color{s.Fg, s.Bg, s.UnderlineColor} {
		b = append(b, byte(c.Kind))
		b = binary.AppendUvarint(b, uint64(c.Value))
	}
	return append(b, byte(s.Underline), byte(s.Attrs))
}

// decoder reads the compact form, remembering the first error so callers
// can check once.
type decoder struct {
	b   []byte
	err error
}

var errShort = errors.New("grid is cut short")

func (d *decoder) byte() byte {
	if d.err != nil {
		return 0
	}
	if len(d.b) == 0 {
		d.err = errShort
		return 0
	}
	v := d.b[0]
	d.b = d.b[1:]
	return v
}

func (d *decoder) bool() bool { return d.byte() != 0 }

func (d *decoder) uvarint() uint64 {
	if d.err != nil {
		return 0
	}
	v, n := binary.Uvarint(d.b)
	if n <= 0 {
		d.err = errShort
		return 0
	}
	d.b = d.b[n:]
	return v
}

func (d *decoder) int() int {
	if d.err != nil {
		return 0
	}
	v, n := binary.Varint(d.b)
	if n <= 0 {
		d.err = errShort
		return 0
	}
	d.b = d.b[n:]
	return int(v)
}

func (d *decoder) string() string {
	n := d.uvarint()
	if d.err != nil {
		return ""
	}
	if n > uint64(len(d.b)) {
		d.err = errShort
		return ""
	}
	s := string(d.b[:n])
	d.b = d.b[n:]
	return s
}

func (d *decoder) style() domain.Style {
	var s domain.Style
	for _, c := range []*domain.Color{&s.Fg, &s.Bg, &s.UnderlineColor} {
		c.Kind = domain.ColorKind(d.byte())
		c.Value = uint32(d.uvarint())
	}
	s.Underline = domain.Underline(d.byte())
	s.Attrs = domain.Attr(d.byte())
	return s
}
