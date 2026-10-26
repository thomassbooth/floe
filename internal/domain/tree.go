package domain

import (
	"fmt"
	"math"
)

// Tree is a tiling layout engine. It is a binary tree whose leaves are
// windows and whose inner nodes split their area between two children along
// an axis.
type Tree struct {
	root   *treeNode
	leaves map[WindowID]*treeNode
}

type treeNode struct {
	parent *treeNode

	// Set on leaves.
	window WindowID

	// Set on splits. ratio is the share of the area given to first.
	axis          Axis
	ratio         float64
	first, second *treeNode
}

func (n *treeNode) isLeaf() bool { return n.first == nil }

var _ Engine = (*Tree)(nil)

// NewTree returns an empty tiling layout.
func NewTree() *Tree {
	return &Tree{leaves: make(map[WindowID]*treeNode)}
}

// Add implements Engine. The target's area is split in half.
func (t *Tree) Add(id, target WindowID, dir Direction) error {
	if _, dup := t.leaves[id]; dup {
		return fmt.Errorf("window %d is already in the layout", id)
	}
	leaf := &treeNode{window: id}
	if t.root == nil {
		t.root = leaf
		t.leaves[id] = leaf
		return nil
	}

	old, ok := t.leaves[target]
	if !ok {
		return fmt.Errorf("split window %d: %w", target, ErrUnknownWindow)
	}
	split := &treeNode{axis: dir.Axis(), ratio: 0.5}
	t.replace(old, split)
	if dir.Forward() {
		split.first, split.second = old, leaf
	} else {
		split.first, split.second = leaf, old
	}
	old.parent, leaf.parent = split, split
	t.leaves[id] = leaf
	return nil
}

// Remove implements Engine. The window's sibling takes over the space the
// two of them shared.
func (t *Tree) Remove(id WindowID) error {
	n, ok := t.leaves[id]
	if !ok {
		return fmt.Errorf("remove window %d: %w", id, ErrUnknownWindow)
	}
	delete(t.leaves, id)

	split := n.parent
	if split == nil {
		t.root = nil
		return nil
	}
	sibling := split.first
	if sibling == n {
		sibling = split.second
	}
	t.replace(split, sibling)
	return nil
}

// Arrange implements Engine.
func (t *Tree) Arrange(area Rect) []Placement {
	out := make([]Placement, 0, len(t.leaves))
	t.walk(t.root, area, func(n *treeNode, r Rect) bool {
		if n.isLeaf() {
			out = append(out, Placement{Window: n.window, Frame: r})
		}
		return true
	})
	return out
}

// Resize implements Engine. It prefers the divider on the dir side of the
// window and falls back to the one on the opposite side, so a window at the
// right edge shrinks when asked to resize right, like tmux.
func (t *Tree) Resize(id WindowID, dir Direction, amount int, area Rect) bool {
	n, ok := t.leaves[id]
	if !ok || amount == 0 {
		return false
	}
	split := t.divider(n, dir.Axis(), dir.Forward())
	if split == nil {
		split = t.divider(n, dir.Axis(), !dir.Forward())
	}
	if split == nil {
		return false
	}

	r, ok := t.rectOf(split, area)
	if !ok {
		return false
	}
	total := r.W
	if split.axis == AxisY {
		total = r.H
	}
	if total <= 0 {
		return false
	}

	delta := amount
	if !dir.Forward() {
		delta = -amount
	}
	current := firstSize(total, split.ratio)
	next := clampFirst(total, current+delta)
	if next == current {
		return false
	}
	split.ratio = float64(next) / float64(total)
	return true
}

// divider finds the nearest split along axis that has n's subtree on its
// first side (when after is true) or its second side.
func (t *Tree) divider(n *treeNode, axis Axis, after bool) *treeNode {
	for child, p := n, n.parent; p != nil; child, p = p, p.parent {
		if p.axis != axis {
			continue
		}
		if (after && p.first == child) || (!after && p.second == child) {
			return p
		}
	}
	return nil
}

// replace puts next where old sits in the tree.
func (t *Tree) replace(old, next *treeNode) {
	p := old.parent
	switch {
	case p == nil:
		t.root = next
	case p.first == old:
		p.first = next
	default:
		p.second = next
	}
	next.parent = p
}

// rectOf returns the area target covers when the tree fills area.
func (t *Tree) rectOf(target *treeNode, area Rect) (Rect, bool) {
	var found Rect
	var ok bool
	t.walk(t.root, area, func(n *treeNode, r Rect) bool {
		if n == target {
			found, ok = r, true
			return false
		}
		return true
	})
	return found, ok
}

// walk visits n and its descendants with the area each covers, stopping when
// visit returns false.
func (t *Tree) walk(n *treeNode, area Rect, visit func(*treeNode, Rect) bool) bool {
	if n == nil {
		return true
	}
	if !visit(n, area) {
		return false
	}
	if n.isLeaf() {
		return true
	}
	a, b := splitRect(area, n.axis, n.ratio)
	return t.walk(n.first, a, visit) && t.walk(n.second, b, visit)
}

func splitRect(r Rect, axis Axis, ratio float64) (Rect, Rect) {
	if axis == AxisX {
		w := firstSize(r.W, ratio)
		return Rect{X: r.X, Y: r.Y, W: w, H: r.H},
			Rect{X: r.X + w, Y: r.Y, W: r.W - w, H: r.H}
	}
	h := firstSize(r.H, ratio)
	return Rect{X: r.X, Y: r.Y, W: r.W, H: h},
		Rect{X: r.X, Y: r.Y + h, W: r.W, H: r.H - h}
}

func firstSize(total int, ratio float64) int {
	return clampFirst(total, int(math.Round(float64(total)*ratio)))
}

// clampFirst keeps both sides of a split at least MinFrame wide when the
// total allows it, and within the total otherwise.
func clampFirst(total, first int) int {
	if total >= 2*MinFrame {
		return min(max(first, MinFrame), total-MinFrame)
	}
	return min(max(first, 0), max(total, 0))
}
