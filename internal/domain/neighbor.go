package domain

// Neighbor finds the window next to from in direction dir. Candidates must lie
// entirely past from's edge and overlap it on the other axis. The closest one
// wins, then the one sharing the longest edge, then the one ranked lowest by
// rank, which lets callers prefer recently focused windows.
func Neighbor(placements []Placement, from WindowID, dir Direction, rank func(WindowID) int) (WindowID, bool) {
	var cur Rect
	found := false
	for _, p := range placements {
		if p.Window == from {
			cur, found = p.Frame, true
			break
		}
	}
	if !found {
		return 0, false
	}

	var best Placement
	bestDist, bestOverlap, bestRank := 0, 0, 0
	ok := false
	for _, p := range placements {
		if p.Window == from {
			continue
		}
		r := p.Frame
		var dist, overlap int
		switch dir {
		case Left:
			dist, overlap = cur.X-r.Right(), span(cur.Y, cur.Bottom(), r.Y, r.Bottom())
		case Right:
			dist, overlap = r.X-cur.Right(), span(cur.Y, cur.Bottom(), r.Y, r.Bottom())
		case Up:
			dist, overlap = cur.Y-r.Bottom(), span(cur.X, cur.Right(), r.X, r.Right())
		case Down:
			dist, overlap = r.Y-cur.Bottom(), span(cur.X, cur.Right(), r.X, r.Right())
		}
		if dist < 0 || overlap <= 0 {
			continue
		}
		rk := rank(p.Window)
		better := !ok ||
			dist < bestDist ||
			(dist == bestDist && overlap > bestOverlap) ||
			(dist == bestDist && overlap == bestOverlap && rk < bestRank)
		if better {
			best, bestDist, bestOverlap, bestRank, ok = p, dist, overlap, rk, true
		}
	}
	return best.Window, ok
}

// span returns how long the ranges [a0, a1) and [b0, b1) overlap.
func span(a0, a1, b0, b1 int) int {
	return min(a1, b1) - max(a0, b0)
}
