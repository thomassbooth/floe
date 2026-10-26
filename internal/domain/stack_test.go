package domain

import (
	"slices"
	"testing"
)

func TestStackOrderAndRaise(t *testing.T) {
	s := NewStack()
	s.Add(1, Rect{W: 10, H: 5})
	s.Add(2, Rect{W: 10, H: 5})
	s.Add(3, Rect{W: 10, H: 5})

	if !slices.Equal(s.IDs(), []WindowID{1, 2, 3}) {
		t.Fatalf("order = %v", s.IDs())
	}
	if !s.Raise(1) || !slices.Equal(s.IDs(), []WindowID{2, 3, 1}) {
		t.Fatalf("after raising 1: %v", s.IDs())
	}
	if s.Raise(1) {
		t.Fatal("raising the top window changes nothing")
	}
	if s.Raise(99) {
		t.Fatal("raising an unknown window changes nothing")
	}

	if !s.Remove(3) || s.Remove(3) {
		t.Fatal("remove should succeed once")
	}
	if !slices.Equal(s.IDs(), []WindowID{2, 1}) || s.Len() != 2 {
		t.Fatalf("after removing 3: %v", s.IDs())
	}
}

func TestStackFrames(t *testing.T) {
	s := NewStack()
	s.Add(1, Rect{X: 1, W: 10, H: 5})
	if s.SetFrame(1, Rect{X: 1, W: 10, H: 5}) {
		t.Fatal("same frame is not a change")
	}
	if !s.SetFrame(1, Rect{X: 2, W: 10, H: 5}) {
		t.Fatal("expected a change")
	}
	if f, _ := s.Frame(1); f.X != 2 {
		t.Fatalf("frame = %+v", f)
	}
	if s.SetFrame(9, Rect{}) {
		t.Fatal("unknown window")
	}
}

func TestFitKeepsWindowsOnScreen(t *testing.T) {
	area := Rect{W: 80, H: 24}
	tests := []struct {
		name string
		in   Rect
		want Rect
	}{
		{"inside stays", Rect{X: 5, Y: 5, W: 20, H: 10}, Rect{X: 5, Y: 5, W: 20, H: 10}},
		{"off the right edge slides back", Rect{X: 70, Y: 0, W: 20, H: 10}, Rect{X: 60, Y: 0, W: 20, H: 10}},
		{"off the top slides down", Rect{X: 0, Y: -3, W: 20, H: 10}, Rect{X: 0, Y: 0, W: 20, H: 10}},
		{"too big shrinks to the area", Rect{X: -5, Y: -5, W: 200, H: 99}, area},
		{"too small grows to the minimum", Rect{X: 0, Y: 0, W: 2, H: 1}, Rect{X: 0, Y: 0, W: MinWindow.W, H: MinWindow.H}},
	}
	for _, tt := range tests {
		if got := fit(tt.in, area); got != tt.want {
			t.Errorf("%s: got %+v, want %+v", tt.name, got, tt.want)
		}
	}
	if got := fit(Rect{W: 20, H: 10}, Rect{W: 5, H: 2}); got != (Rect{W: 5, H: 2}) {
		t.Errorf("an area smaller than the minimum wins: got %+v", got)
	}
}

func TestScaleKeepsTouchingWindowsTouching(t *testing.T) {
	from := Rect{W: 80, H: 24}
	to := Rect{W: 123, H: 37}
	left := scale(Rect{W: 40, H: 24}, from, to)
	right := scale(Rect{X: 40, W: 40, H: 24}, from, to)
	if left.Right() != right.X || right.Right() != to.W || left.H != to.H {
		t.Fatalf("left %+v, right %+v", left, right)
	}
}
