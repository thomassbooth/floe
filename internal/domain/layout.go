package domain

import (
	"errors"
	"math"
)

// ErrUnknownWindow is returned when a session is asked about a window it
// does not hold.
var ErrUnknownWindow = errors.New("unknown window")

// FrameInset is how many cells a window's frame takes on each side of its
// content.
const FrameInset = 1

// MinWindow is the smallest a window may be made, room for its frame and a
// short title.
var MinWindow = Size{W: 12, H: 3}

// Placement is where a window is drawn.
type Placement struct {
	Window WindowID
	Frame  Rect
}

// fit keeps r inside area: no bigger than the area, no smaller than
// MinWindow where the area allows, and fully on screen.
func fit(r, area Rect) Rect {
	w := clamp(r.W, min(MinWindow.W, area.W), area.W)
	h := clamp(r.H, min(MinWindow.H, area.H), area.H)
	return Rect{
		X: clamp(r.X, area.X, area.Right()-w),
		Y: clamp(r.Y, area.Y, area.Bottom()-h),
		W: w,
		H: h,
	}
}

// split divides r in two along dir's axis. keep is the part on the side
// opposite dir and open the part on the dir side. It fails when either part
// would be smaller than MinWindow.
func split(r Rect, dir Direction) (keep, open Rect, ok bool) {
	if dir.Axis() == AxisX {
		first := (r.W + 1) / 2
		a := Rect{X: r.X, Y: r.Y, W: first, H: r.H}
		b := Rect{X: r.X + first, Y: r.Y, W: r.W - first, H: r.H}
		if b.W < MinWindow.W {
			return Rect{}, Rect{}, false
		}
		if dir == Left {
			return b, a, true
		}
		return a, b, true
	}
	first := (r.H + 1) / 2
	a := Rect{X: r.X, Y: r.Y, W: r.W, H: first}
	b := Rect{X: r.X, Y: r.Y + first, W: r.W, H: r.H - first}
	if b.H < MinWindow.H {
		return Rect{}, Rect{}, false
	}
	if dir == Up {
		return b, a, true
	}
	return a, b, true
}

// grow extends r across the neighboring rectangle gone, which shares one of
// r's edges.
func grow(r, gone Rect) Rect {
	switch {
	case gone.Right() == r.X:
		return Rect{X: gone.X, Y: r.Y, W: r.W + gone.W, H: r.H}
	case gone.X == r.Right():
		return Rect{X: r.X, Y: r.Y, W: r.W + gone.W, H: r.H}
	case gone.Bottom() == r.Y:
		return Rect{X: r.X, Y: gone.Y, W: r.W, H: r.H + gone.H}
	default:
		return Rect{X: r.X, Y: r.Y, W: r.W, H: r.H + gone.H}
	}
}

// scale maps r from one area to another proportionally. Edges are scaled
// rather than sizes so windows that touched before still touch after.
func scale(r, from, to Rect) Rect {
	sx := func(x int) int {
		return to.X + int(math.Round(float64(x-from.X)*float64(to.W)/float64(from.W)))
	}
	sy := func(y int) int {
		return to.Y + int(math.Round(float64(y-from.Y)*float64(to.H)/float64(from.H)))
	}
	x0, y0 := sx(r.X), sy(r.Y)
	return Rect{X: x0, Y: y0, W: sx(r.Right()) - x0, H: sy(r.Bottom()) - y0}
}

// clamp limits v to [lo, hi]. When the range is empty, hi wins.
func clamp(v, lo, hi int) int {
	return min(max(v, lo), hi)
}
