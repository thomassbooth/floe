package domain

import "math"

// Zone is a part of the screen a window can be snapped to.
type Zone int

const (
	ZoneNone Zone = iota
	ZoneLeft
	ZoneRight
	ZoneTopLeft
	ZoneTopRight
	ZoneBottomLeft
	ZoneBottomRight
)

// ZoneAt returns the zone a window dragged with the pointer at p snaps to.
// Only the left and right edges of the area snap, so a drop anywhere else
// places the window freely, and the top row stays free for title bars. The
// top or bottom quarter of an edge picks a corner quarter, the rest of it a
// half. Positions past the area, such as on the status bar below it, count
// as its nearest edge.
func ZoneAt(p Point, area Rect) Zone {
	if area.Empty() {
		return ZoneNone
	}
	var left bool
	switch {
	case p.X <= area.X:
		left = true
	case p.X >= area.Right()-1:
		left = false
	default:
		return ZoneNone
	}
	band := max(area.H/4, 1)
	switch {
	case p.Y < area.Y+band:
		if left {
			return ZoneTopLeft
		}
		return ZoneTopRight
	case p.Y >= area.Bottom()-band:
		if left {
			return ZoneBottomLeft
		}
		return ZoneBottomRight
	case left:
		return ZoneLeft
	default:
		return ZoneRight
	}
}

// Rect returns the part of area the zone covers. Odd sizes give the extra
// cell to the left and top parts, matching how splits divide a window.
func (z Zone) Rect(area Rect) Rect {
	w := (area.W + 1) / 2
	h := (area.H + 1) / 2
	left := Rect{X: area.X, Y: area.Y, W: w, H: area.H}
	right := Rect{X: area.X + w, Y: area.Y, W: area.W - w, H: area.H}
	top := func(r Rect) Rect { return Rect{X: r.X, Y: r.Y, W: r.W, H: h} }
	bottom := func(r Rect) Rect { return Rect{X: r.X, Y: r.Y + h, W: r.W, H: r.H - h} }
	switch z {
	case ZoneLeft:
		return left
	case ZoneRight:
		return right
	case ZoneTopLeft:
		return top(left)
	case ZoneTopRight:
		return top(right)
	case ZoneBottomLeft:
		return bottom(left)
	case ZoneBottomRight:
		return bottom(right)
	}
	return Rect{}
}

// Tile returns n rectangles that cover area together, no gaps or overlaps:
// the whole area for one window, halves for two, a half and two quarters
// for three, and beyond that a grid of near-square cells whose last row
// stretches to stay full.
func Tile(n int, area Rect) []Rect {
	switch {
	case n <= 0:
		return nil
	case n == 1:
		return []Rect{area}
	case n == 2:
		return []Rect{ZoneLeft.Rect(area), ZoneRight.Rect(area)}
	case n == 3:
		return []Rect{ZoneLeft.Rect(area), ZoneTopRight.Rect(area), ZoneBottomRight.Rect(area)}
	}

	cols := int(math.Ceil(math.Sqrt(float64(n))))
	rows := (n + cols - 1) / cols
	out := make([]Rect, 0, n)
	for r := range rows {
		inRow := min(cols, n-r*cols)
		y0, y1 := edge(area.Y, area.H, r, rows), edge(area.Y, area.H, r+1, rows)
		for c := range inRow {
			x0, x1 := edge(area.X, area.W, c, inRow), edge(area.X, area.W, c+1, inRow)
			out = append(out, Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0})
		}
	}
	return out
}

// edge returns where the i-th of k equal parts of a span starts, so the
// parts always add up to the whole span.
func edge(start, length, i, k int) int {
	return start + int(math.Round(float64(i*length)/float64(k)))
}
