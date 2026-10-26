package domain

import "testing"

func TestZoneAt(t *testing.T) {
	area := Rect{W: 80, H: 24} // corner bands are rows 0-5 and 18-23
	tests := []struct {
		name string
		p    Point
		want Zone
	}{
		{"middle of the screen", Point{X: 40, Y: 12}, ZoneNone},
		{"near but not at the left edge", Point{X: 1, Y: 12}, ZoneNone},
		{"left edge", Point{X: 0, Y: 12}, ZoneLeft},
		{"right edge", Point{X: 79, Y: 12}, ZoneRight},
		{"past the right edge", Point{X: 85, Y: 12}, ZoneRight},
		{"top of the left edge", Point{X: 0, Y: 0}, ZoneTopLeft},
		{"just inside the top band", Point{X: 0, Y: 5}, ZoneTopLeft},
		{"just below the top band", Point{X: 0, Y: 6}, ZoneLeft},
		{"bottom of the right edge", Point{X: 79, Y: 23}, ZoneBottomRight},
		{"on the status bar below", Point{X: 0, Y: 24}, ZoneBottomLeft},
		{"top row away from the edges", Point{X: 40, Y: 0}, ZoneNone},
	}
	for _, tt := range tests {
		if got := ZoneAt(tt.p, area); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
	if got := ZoneAt(Point{}, Rect{}); got != ZoneNone {
		t.Errorf("an empty area has no zones, got %v", got)
	}
}

func TestZoneRect(t *testing.T) {
	area := Rect{X: 1, Y: 2, W: 81, H: 25}
	tests := map[Zone]Rect{
		ZoneLeft:        {X: 1, Y: 2, W: 41, H: 25},
		ZoneRight:       {X: 42, Y: 2, W: 40, H: 25},
		ZoneTopLeft:     {X: 1, Y: 2, W: 41, H: 13},
		ZoneTopRight:    {X: 42, Y: 2, W: 40, H: 13},
		ZoneBottomLeft:  {X: 1, Y: 15, W: 41, H: 12},
		ZoneBottomRight: {X: 42, Y: 15, W: 40, H: 12},
		ZoneNone:        {},
	}
	for z, want := range tests {
		if got := z.Rect(area); got != want {
			t.Errorf("zone %v: got %+v, want %+v", z, got, want)
		}
	}
}

func TestTileCoversTheAreaExactly(t *testing.T) {
	for _, area := range []Rect{{W: 80, H: 24}, {X: 3, Y: 1, W: 101, H: 37}, {W: 13, H: 7}} {
		for n := 1; n <= 12; n++ {
			rects := Tile(n, area)
			if len(rects) != n {
				t.Fatalf("n=%d: got %d rects", n, len(rects))
			}
			covered := make([]int, area.W*area.H)
			for _, r := range rects {
				if r.Empty() {
					t.Fatalf("n=%d area %v: empty rect %+v", n, area, r)
				}
				for y := r.Y; y < r.Bottom(); y++ {
					for x := r.X; x < r.Right(); x++ {
						covered[(y-area.Y)*area.W+(x-area.X)]++
					}
				}
			}
			for i, c := range covered {
				if c != 1 {
					t.Fatalf("n=%d area %v: cell %d covered %d times", n, area, i, c)
				}
			}
		}
	}
}

func TestTileShapes(t *testing.T) {
	area := Rect{W: 80, H: 24}
	three := Tile(3, area)
	if three[0] != (Rect{W: 40, H: 24}) || three[1] != (Rect{X: 40, W: 40, H: 12}) || three[2] != (Rect{X: 40, Y: 12, W: 40, H: 12}) {
		t.Fatalf("three = %+v", three)
	}
	five := Tile(5, area) // 3 on top, 2 stretched below
	if five[0].W != 27 || five[3] != (Rect{Y: 12, W: 40, H: 12}) || five[4] != (Rect{X: 40, Y: 12, W: 40, H: 12}) {
		t.Fatalf("five = %+v", five)
	}
}

func TestSessionSnapRemembersFreeFrame(t *testing.T) {
	s := newSession(t)
	free := Rect{X: 10, Y: 4, W: 30, H: 10}
	s.Place(1, free)

	if !s.Snap(1, ZoneLeft) {
		t.Fatal("expected a change")
	}
	w := s.Snapshot().Windows[0]
	if w.Frame != (Rect{W: 40, H: 24}) || w.Restore != free || w.Maximized {
		t.Fatalf("snapped window = %+v", w)
	}

	// Snapping again keeps the original free frame.
	s.Snap(1, ZoneBottomRight)
	if w := s.Snapshot().Windows[0]; w.Frame != (Rect{X: 40, Y: 12, W: 40, H: 12}) || w.Restore != free {
		t.Fatalf("re-snapped window = %+v", w)
	}

	// Maximizing a snapped window and restoring it goes back to free.
	s.ToggleMaximize(1)
	if w := s.Snapshot().Windows[0]; !w.Maximized {
		t.Fatalf("expected maximized, got %+v", w)
	}
	s.ToggleMaximize(1)
	if w := s.Snapshot().Windows[0]; w.Frame != free || w.Restore != (Rect{}) {
		t.Fatalf("restored window = %+v", w)
	}

	// Moving a snapped window frees it.
	s.Snap(1, ZoneRight)
	s.Place(1, Rect{X: 5, Y: 5, W: 30, H: 10})
	if w := s.Snapshot().Windows[0]; w.Restore != (Rect{}) {
		t.Fatalf("moved window should be free, got %+v", w)
	}
	if s.Snap(1, ZoneNone) {
		t.Fatal("ZoneNone is not a snap")
	}
}

func TestSessionTidyFillsTheScreenKeepingPositions(t *testing.T) {
	s := newSession(t)
	s.OpenAt("sh", 2, Rect{X: 50, Y: 2, W: 20, H: 8})  // top right
	s.OpenAt("sh", 3, Rect{X: 50, Y: 14, W: 20, H: 8}) // bottom right
	s.Place(1, Rect{X: 2, Y: 3, W: 30, H: 15})         // left

	if !s.Tidy() {
		t.Fatal("expected a change")
	}
	got := frames(s)
	if got[1] != (Rect{W: 40, H: 24}) || got[2] != (Rect{X: 40, W: 40, H: 12}) || got[3] != (Rect{X: 40, Y: 12, W: 40, H: 12}) {
		t.Fatalf("tidied = %v", got)
	}
	if s.Tidy() {
		t.Fatal("tidying a tidy screen changes nothing")
	}
	if w := s.Snapshot().Windows[0]; w.Restore == (Rect{}) {
		t.Fatal("tidied windows remember their free frames")
	}
}
