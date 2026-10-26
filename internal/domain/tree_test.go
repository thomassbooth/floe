package domain

import (
	"errors"
	"slices"
	"testing"
)

var screen = Rect{W: 80, H: 24}

func placements(t *testing.T, tr *Tree, area Rect) map[WindowID]Rect {
	t.Helper()
	out := map[WindowID]Rect{}
	for _, p := range tr.Arrange(area) {
		out[p.Window] = p.Frame
	}
	return out
}

func TestTreeArrange(t *testing.T) {
	tests := []struct {
		name  string
		build func(*Tree)
		area  Rect
		want  map[WindowID]Rect
	}{
		{
			name:  "empty",
			build: func(*Tree) {},
			area:  screen,
			want:  map[WindowID]Rect{},
		},
		{
			name:  "single window fills the area",
			build: func(tr *Tree) { tr.Add(1, 0, Right) },
			area:  screen,
			want:  map[WindowID]Rect{1: screen},
		},
		{
			name: "split right halves the width",
			build: func(tr *Tree) {
				tr.Add(1, 0, Right)
				tr.Add(2, 1, Right)
			},
			area: screen,
			want: map[WindowID]Rect{
				1: {X: 0, Y: 0, W: 40, H: 24},
				2: {X: 40, Y: 0, W: 40, H: 24},
			},
		},
		{
			name: "split left puts the new window first",
			build: func(tr *Tree) {
				tr.Add(1, 0, Right)
				tr.Add(2, 1, Left)
			},
			area: screen,
			want: map[WindowID]Rect{
				2: {X: 0, Y: 0, W: 40, H: 24},
				1: {X: 40, Y: 0, W: 40, H: 24},
			},
		},
		{
			name: "odd sizes round the first window up",
			build: func(tr *Tree) {
				tr.Add(1, 0, Down)
				tr.Add(2, 1, Down)
			},
			area: Rect{X: 3, Y: 2, W: 10, H: 7},
			want: map[WindowID]Rect{
				1: {X: 3, Y: 2, W: 10, H: 4},
				2: {X: 3, Y: 6, W: 10, H: 3},
			},
		},
		{
			name: "nested splits",
			build: func(tr *Tree) {
				tr.Add(1, 0, Right)
				tr.Add(2, 1, Right)
				tr.Add(3, 2, Down)
			},
			area: screen,
			want: map[WindowID]Rect{
				1: {X: 0, Y: 0, W: 40, H: 24},
				2: {X: 40, Y: 0, W: 40, H: 12},
				3: {X: 40, Y: 12, W: 40, H: 12},
			},
		},
		{
			name: "tiny areas never produce negative sizes",
			build: func(tr *Tree) {
				tr.Add(1, 0, Right)
				tr.Add(2, 1, Right)
			},
			area: Rect{W: 1, H: 1},
			want: map[WindowID]Rect{
				1: {X: 0, Y: 0, W: 1, H: 1},
				2: {X: 1, Y: 0, W: 0, H: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTree()
			tt.build(tr)
			got := placements(t, tr, tt.area)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d placements, want %d: %v", len(got), len(tt.want), got)
			}
			for id, want := range tt.want {
				if got[id] != want {
					t.Errorf("window %d: got %+v, want %+v", id, got[id], want)
				}
			}
		})
	}
}

func TestTreeArrangeTilesWithoutGapsOrOverlap(t *testing.T) {
	tr := NewTree()
	tr.Add(1, 0, Right)
	tr.Add(2, 1, Right)
	tr.Add(3, 2, Down)
	tr.Add(4, 1, Down)
	tr.Add(5, 4, Right)

	for _, area := range []Rect{screen, {W: 13, H: 7}, {W: 200, H: 61}} {
		covered := make([]int, area.W*area.H)
		for _, p := range tr.Arrange(area) {
			for y := p.Frame.Y; y < p.Frame.Bottom(); y++ {
				for x := p.Frame.X; x < p.Frame.Right(); x++ {
					covered[y*area.W+x]++
				}
			}
		}
		for i, n := range covered {
			if n != 1 {
				t.Fatalf("area %v: cell (%d,%d) covered %d times", area, i%area.W, i/area.W, n)
			}
		}
	}
}

func TestTreeRemoveGivesSpaceToSibling(t *testing.T) {
	tr := NewTree()
	tr.Add(1, 0, Right)
	tr.Add(2, 1, Right)
	tr.Add(3, 2, Down)

	if err := tr.Remove(2); err != nil {
		t.Fatal(err)
	}
	got := placements(t, tr, screen)
	want := map[WindowID]Rect{
		1: {X: 0, Y: 0, W: 40, H: 24},
		3: {X: 40, Y: 0, W: 40, H: 24},
	}
	if len(got) != 2 || got[1] != want[1] || got[3] != want[3] {
		t.Fatalf("got %v, want %v", got, want)
	}

	tr.Remove(1)
	tr.Remove(3)
	if got := tr.Arrange(screen); len(got) != 0 {
		t.Fatalf("expected empty layout, got %v", got)
	}
}

func TestTreeErrors(t *testing.T) {
	tr := NewTree()
	tr.Add(1, 0, Right)
	if err := tr.Add(1, 1, Right); err == nil {
		t.Error("adding a duplicate window should fail")
	}
	if err := tr.Add(2, 99, Right); !errors.Is(err, ErrUnknownWindow) {
		t.Errorf("splitting an unknown window: got %v", err)
	}
	if err := tr.Remove(99); !errors.Is(err, ErrUnknownWindow) {
		t.Errorf("removing an unknown window: got %v", err)
	}
}

func TestTreeResize(t *testing.T) {
	newPair := func() *Tree {
		tr := NewTree()
		tr.Add(1, 0, Right)
		tr.Add(2, 1, Right)
		return tr
	}

	tests := []struct {
		name      string
		window    WindowID
		dir       Direction
		amount    int
		changed   bool
		wantWidth int // of window 1
	}{
		{"left window grows right", 1, Right, 5, true, 45},
		{"left window shrinks left", 1, Left, 5, true, 35},
		{"right window grows left", 2, Left, 5, true, 35},
		{"right window at the edge shrinks right", 2, Right, 5, true, 45},
		{"clamps to the minimum frame", 1, Left, 100, true, MinFrame},
		{"clamps the other side too", 1, Right, 100, true, 80 - MinFrame},
		{"wrong axis has no divider", 1, Up, 5, false, 40},
		{"zero amount is a no-op", 1, Right, 0, false, 40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newPair()
			if got := tr.Resize(tt.window, tt.dir, tt.amount, screen); got != tt.changed {
				t.Errorf("changed = %v, want %v", got, tt.changed)
			}
			if got := placements(t, tr, screen)[1].W; got != tt.wantWidth {
				t.Errorf("window 1 width = %d, want %d", got, tt.wantWidth)
			}
		})
	}
}

func TestTreeResizeKeepsProportionsWhenAreaChanges(t *testing.T) {
	tr := NewTree()
	tr.Add(1, 0, Right)
	tr.Add(2, 1, Right)
	tr.Resize(1, Right, 20, screen) // 60 of 80

	got := placements(t, tr, Rect{W: 160, H: 24})
	if got[1].W != 120 {
		t.Fatalf("window 1 width = %d, want 120", got[1].W)
	}
}

func TestTreeResizeUsesNearestMatchingDivider(t *testing.T) {
	// 1 | (2 / 3): resizing 3 horizontally moves the outer divider.
	tr := NewTree()
	tr.Add(1, 0, Right)
	tr.Add(2, 1, Right)
	tr.Add(3, 2, Down)

	if !tr.Resize(3, Left, 10, screen) {
		t.Fatal("expected resize to change the layout")
	}
	got := placements(t, tr, screen)
	if got[1].W != 30 || got[2].W != 50 || got[3].W != 50 {
		t.Fatalf("got %v", got)
	}
}

func TestTreeArrangeOrderIsStable(t *testing.T) {
	tr := NewTree()
	tr.Add(1, 0, Right)
	tr.Add(2, 1, Right)
	tr.Add(3, 1, Down)

	var ids []WindowID
	for _, p := range tr.Arrange(screen) {
		ids = append(ids, p.Window)
	}
	if !slices.Equal(ids, []WindowID{1, 3, 2}) {
		t.Fatalf("got order %v", ids)
	}
}
