package ui

import (
	"testing"
	"time"

	"github.com/thomassbooth/floe/internal/domain"
)

var area = domain.Rect{W: 100, H: 30}

func snapOf(frames ...domain.Rect) domain.Snapshot {
	s := domain.Snapshot{Session: 1, Area: area}
	for i, f := range frames {
		s.Windows = append(s.Windows, domain.WindowView{ID: domain.WindowID(i + 1), Frame: f, Title: "sh"})
	}
	return s
}

var (
	full  = domain.Rect{W: 100, H: 30}
	left  = domain.Rect{W: 50, H: 30}
	right = domain.Rect{X: 50, W: 50, H: 30}
)

// settled returns a model showing s with nothing moving.
func settled(s domain.Snapshot) Model {
	return Model{animate: true, snap: s}
}

// at moves m's animation to d after it started.
func (m *Model) at(d time.Duration) {
	m.anim.now = m.anim.start.Add(d)
}

func TestLerpRectMovesEdges(t *testing.T) {
	if got := lerpRect(full, left, 0); got != full {
		t.Errorf("at 0: %v", got)
	}
	if got := lerpRect(full, left, 1); got != left {
		t.Errorf("at 1: %v", got)
	}
	// Only the right edge moves; the left edge stays at 0 throughout.
	if got := lerpRect(full, left, 0.5); got != (domain.Rect{W: 75, H: 30}) {
		t.Errorf("halfway: %v", got)
	}
}

func TestSplitAnimatesBothWindows(t *testing.T) {
	m := settled(snapOf(full))
	if cmd := m.setLayout(snapOf(left, right)); cmd == nil || m.anim == nil {
		t.Fatal("a split should start an animation")
	}
	w1, w2 := m.snap.Windows[0], m.snap.Windows[1]
	if got := m.drawnFrame(w1); got != full {
		t.Errorf("window 1 should start where it was: %v", got)
	}
	if got := m.drawnFrame(w2); got != seedFrame(right) {
		t.Errorf("window 2 should start small in its spot: %v", got)
	}

	m.at(animDuration / 2)
	if got := m.drawnFrame(w1); got.W <= left.W || got.W >= full.W {
		t.Errorf("halfway, window 1 should be between sizes: %v", got)
	}
	if v := m.drawnView(w1); v.Content != v.Frame.Inset(domain.FrameInset) {
		t.Errorf("content should sit inside the moving frame: %+v", v)
	}

	if m.stepAnimation(m.anim.start.Add(animDuration)) != nil || m.anim != nil {
		t.Fatal("the animation should end on time")
	}
	if got := m.drawnFrame(w1); got != left {
		t.Errorf("window 1 should end in place: %v", got)
	}
}

func TestSmallMovesJump(t *testing.T) {
	m := settled(snapOf(left, right))
	nudged := snapOf(domain.Rect{W: 52, H: 30}, domain.Rect{X: 52, W: 48, H: 29})
	if m.setLayout(nudged); m.anim != nil {
		t.Fatal("a nudge of two cells should not animate")
	}
}

func TestRedrawsDoNotRestartAnAnimation(t *testing.T) {
	m := settled(snapOf(full))
	m.setLayout(snapOf(left, right))
	m.at(animDuration / 2)
	anim := m.anim

	retitled := snapOf(left, right)
	retitled.Windows[0].Title = "vim"
	if m.setLayout(retitled); m.anim != anim {
		t.Fatal("a new title should leave the animation running")
	}
}

func TestNewLayoutMidwayStartsFromWhereWindowsAre(t *testing.T) {
	m := settled(snapOf(full))
	m.setLayout(snapOf(left, right))
	m.at(animDuration / 2)
	midway := m.drawnFrame(m.snap.Windows[0])

	if cmd := m.setLayout(snapOf(full)); cmd != nil {
		t.Error("the running animation's ticks should carry on, not start again")
	}
	if got := m.drawnFrame(m.snap.Windows[0]); got != midway {
		t.Fatalf("window 1 should carry on from %v, not jump to %v", midway, got)
	}
}

func TestNoAnimationWhen(t *testing.T) {
	for name, tc := range map[string]struct {
		m    Model
		next domain.Snapshot
	}{
		"turned off":        {Model{snap: snapOf(full)}, snapOf(left, right)},
		"attaching":         {settled(domain.Snapshot{}), snapOf(left, right)},
		"dragging":          {Model{animate: true, snap: snapOf(full), drag: &drag{}}, snapOf(left)},
		"switching session": {settled(snapOf(full)), func() domain.Snapshot { s := snapOf(left, right); s.Session = 2; return s }()},
		"resizing the terminal": {settled(snapOf(full)), func() domain.Snapshot {
			s := snapOf(domain.Rect{W: 80, H: 30})
			s.Area.W = 80
			return s
		}()},
	} {
		t.Run(name, func(t *testing.T) {
			m := tc.m
			if m.setLayout(tc.next); m.anim != nil {
				t.Fatal("should not animate")
			}
			if len(m.snap.Windows) != len(tc.next.Windows) {
				t.Fatal("the new layout should still show")
			}
		})
	}
}
