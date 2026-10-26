package domain

import "errors"

// ErrUnknownWindow is returned when a layout is asked about a window it does
// not hold.
var ErrUnknownWindow = errors.New("unknown window")

// FrameInset is how many cells a window's frame takes on each side of its
// content.
const FrameInset = 1

// MinFrame is the smallest width or height a layout gives a window when
// there is room: a frame plus one cell of content.
const MinFrame = 2*FrameInset + 1

// Placement is where a layout engine puts a window.
type Placement struct {
	Window WindowID
	Frame  Rect
}

// Engine arranges a set of windows within an area. Engines are pure: given
// the same windows and area, Arrange always returns the same placements.
type Engine interface {
	// Add inserts a window. When the engine already holds windows, the new
	// one is placed on the dir side of target.
	Add(id, target WindowID, dir Direction) error
	// Remove takes a window out and gives its space to its neighbors.
	Remove(id WindowID) error
	// Arrange returns a placement for every window, back to front.
	Arrange(area Rect) []Placement
	// Resize moves the window's nearest divider amount cells in direction
	// dir. It reports whether anything changed.
	Resize(id WindowID, dir Direction, amount int, area Rect) bool
}
