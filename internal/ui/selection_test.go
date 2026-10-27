package ui

import (
	"strings"
	"testing"

	"github.com/thomassbooth/floe/internal/domain"
)

// gridOf builds a grid from lines of text, padded to the widest line.
func gridOf(lines ...string) domain.Grid {
	w := 0
	for _, l := range lines {
		w = max(w, len(l))
	}
	g := domain.Grid{Size: domain.Size{W: w, H: len(lines)}, Cells: make([]domain.Cell, w*len(lines))}
	for y, l := range lines {
		l += strings.Repeat(" ", w-len(l))
		for x, r := range l {
			g.Cells[y*w+x] = domain.Cell{Content: string(r), Width: 1}
		}
	}
	return g
}

func sel(ax, ay, hx, hy int) selection {
	return selection{anchor: domain.Point{X: ax, Y: ay}, head: domain.Point{X: hx, Y: hy}}
}

func TestSelectedText(t *testing.T) {
	g := gridOf(
		"first line here",
		"second line",
		"third line here",
	)
	tests := []struct {
		name string
		s    selection
		want string
	}{
		{"within one line", sel(6, 0, 9, 0), "line"},
		{"backwards within a line", sel(9, 0, 6, 0), "line"},
		{"across lines runs like text", sel(6, 0, 5, 2), "line here\nsecond line\nthird"},
		{"dragged upwards", sel(5, 2, 6, 0), "line here\nsecond line\nthird"},
		{"trailing blanks are dropped", sel(7, 1, 14, 1), "line"},
	}
	for _, tt := range tests {
		if got := selectedText(g, tt.s); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestWordAtAndLineEnd(t *testing.T) {
	g := gridOf(`cd ~/Projects/mux && "quoted" x`)
	tests := []struct {
		x        int
		from, to int
	}{
		{0, 0, 1},    // cd
		{8, 3, 16},   // ~/Projects/mux
		{23, 22, 27}, // quoted, without its quotes
		{2, 2, 2},    // a space selects just itself
	}
	for _, tt := range tests {
		if from, to := wordAt(g, tt.x, 0); from != tt.from || to != tt.to {
			t.Errorf("word at %d: got %d-%d, want %d-%d", tt.x, from, to, tt.from, tt.to)
		}
	}
	if got := lineEnd(gridOf("abc   ", "      "), 0); got != 2 {
		t.Errorf("line end = %d, want 2", got)
	}
}

func TestSelectionContains(t *testing.T) {
	s := sel(5, 1, 2, 3) // from (5,1) to (2,3)
	cases := map[[2]int]bool{
		{4, 1}: false, {5, 1}: true, {99, 1}: true,
		{0, 2}: true,
		{2, 3}: true, {3, 3}: false,
		{0, 0}: false, {0, 4}: false,
	}
	for p, want := range cases {
		if got := s.contains(p[0], p[1]); got != want {
			t.Errorf("contains(%d,%d) = %v, want %v", p[0], p[1], got, want)
		}
	}
}
