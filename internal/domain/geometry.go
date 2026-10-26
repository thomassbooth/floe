// Package domain holds the multiplexer's core model: sessions, windows,
// layout engines, commands and events. It is pure Go with no IO and no
// third-party imports, so every rule here is testable without a terminal.
package domain

// Point is a cell position, with X growing right and Y growing down.
type Point struct {
	X, Y int
}

// Size is a width and height in cells.
type Size struct {
	W, H int
}

// Rect is an area of cells with its top-left corner at X, Y.
type Rect struct {
	X, Y, W, H int
}

// Right returns the X coordinate just past the rectangle's right edge.
func (r Rect) Right() int { return r.X + r.W }

// Bottom returns the Y coordinate just past the rectangle's bottom edge.
func (r Rect) Bottom() int { return r.Y + r.H }

// Size returns the rectangle's dimensions.
func (r Rect) Size() Size { return Size{W: r.W, H: r.H} }

// Empty reports whether the rectangle covers no cells.
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// Contains reports whether the cell at x, y lies inside the rectangle.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.Right() && y >= r.Y && y < r.Bottom()
}

// Inset shrinks the rectangle by n cells on every side. The result never has
// a negative size.
func (r Rect) Inset(n int) Rect {
	out := Rect{X: r.X + n, Y: r.Y + n, W: r.W - 2*n, H: r.H - 2*n}
	out.W = max(out.W, 0)
	out.H = max(out.H, 0)
	return out
}

// Axis is the orientation along which an area is divided.
type Axis int

const (
	// AxisX divides an area into a left and a right part.
	AxisX Axis = iota
	// AxisY divides an area into a top and a bottom part.
	AxisY
)

// Direction is one of the four screen directions.
type Direction int

const (
	Left Direction = iota
	Right
	Up
	Down
)

// Axis returns the axis a direction moves along.
func (d Direction) Axis() Axis {
	if d == Left || d == Right {
		return AxisX
	}
	return AxisY
}

// Forward reports whether the direction moves toward larger coordinates.
func (d Direction) Forward() bool {
	return d == Right || d == Down
}

func (d Direction) String() string {
	switch d {
	case Left:
		return "left"
	case Right:
		return "right"
	case Up:
		return "up"
	case Down:
		return "down"
	}
	return "unknown"
}
