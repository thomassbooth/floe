package ui

import (
	"math"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/thomassbooth/floe/internal/domain"
)

// animDuration is how long windows take to move into a new layout: long
// enough to follow, short enough never to feel like waiting.
const animDuration = 150 * time.Millisecond

// animNudge is the most any edge of a window may move without animating.
// Steps this small, like a keyboard resize, read better as a jump.
const animNudge = 2

// animTickMsg moves the running animation on by a frame.
type animTickMsg struct{ at time.Time }

// animation moves windows from where they were drawn to where the layout
// puts them now. Only frames move: programs get their new size at once.
type animation struct {
	start time.Time
	// now is the time of the frame on screen.
	now time.Time
	// from is where each window started. New windows start small, in the
	// middle of where they are going.
	from map[domain.WindowID]domain.Rect
}

// frameOf returns where to draw a window that is on its way to target.
func (a *animation) frameOf(id domain.WindowID, target domain.Rect) domain.Rect {
	from, ok := a.from[id]
	if !ok {
		return target
	}
	return lerpRect(from, target, easeOut(a.progress()))
}

func (a *animation) progress() float64 {
	return min(max(float64(a.now.Sub(a.start))/float64(animDuration), 0), 1)
}

func (a *animation) done() bool { return a.progress() >= 1 }

// easeOut starts fast and settles gently, so a window answers the key at
// once and then slows into place.
func easeOut(p float64) float64 { return 1 - math.Pow(1-p, 3) }

// lerpRect returns the rectangle p of the way from a to b. It moves the
// edges rather than the position and size, so an edge that stays put does
// not wobble from rounding.
func lerpRect(a, b domain.Rect, p float64) domain.Rect {
	lerp := func(x, y int) int { return int(math.Round(float64(x) + float64(y-x)*p)) }
	left, top := lerp(a.X, b.X), lerp(a.Y, b.Y)
	right, bottom := lerp(a.Right(), b.Right()), lerp(a.Bottom(), b.Bottom())
	return domain.Rect{X: left, Y: top, W: right - left, H: bottom - top}
}

// seedFrame is where a new window starts: half its size, centered on where
// it is going, so it grows into place.
func seedFrame(r domain.Rect) domain.Rect {
	w, h := r.W/2, r.H/2
	return domain.Rect{X: r.X + (r.W-w)/2, Y: r.Y + (r.H-h)/2, W: w, H: h}
}

// farApart reports whether some edge moved more than a nudge.
func farApart(a, b domain.Rect) bool {
	d := func(x, y int) bool { return x-y > animNudge || y-x > animNudge }
	return d(a.X, b.X) || d(a.Y, b.Y) || d(a.Right(), b.Right()) || d(a.Bottom(), b.Bottom())
}

// setLayout shows a new snapshot, animating the windows into it when the
// layout really changed. Windows follow the mouse while dragged, and jump
// when the terminal or the session changes, since there is nothing to
// follow then.
func (m *Model) setLayout(next domain.Snapshot) tea.Cmd {
	prev := m.snap
	m.snap = next
	if !m.animate || m.drag != nil || len(prev.Windows) == 0 ||
		prev.Session != next.Session || prev.Area != next.Area {
		m.anim = nil
		return nil
	}

	// A change that only redraws, such as a new title, or only nudges a
	// window, leaves a running animation alone: restarting it would hold
	// windows back each time a busy program updates its title.
	// Minimized windows are left out, so one coming back grows into place
	// like a new window.
	targets := make(map[domain.WindowID]domain.Rect, len(prev.Windows))
	for _, w := range prev.Windows {
		if !w.Minimized {
			targets[w.ID] = w.Frame
		}
	}
	moved := false
	for _, w := range next.Windows {
		if w.Minimized {
			continue
		}
		if t, ok := targets[w.ID]; !ok || farApart(t, w.Frame) {
			moved = true
			break
		}
	}
	if !moved {
		return nil
	}

	// Start from where windows are drawn now, which is partway along when
	// an animation is already running.
	from := make(map[domain.WindowID]domain.Rect, len(next.Windows))
	for _, w := range prev.Windows {
		if !w.Minimized {
			from[w.ID] = m.drawnFrame(w)
		}
	}
	for _, w := range next.Windows {
		if _, ok := from[w.ID]; !ok {
			from[w.ID] = seedFrame(w.Frame)
		}
	}
	now := time.Now()
	running := m.anim != nil
	m.anim = &animation{start: now, now: now, from: from}
	if running {
		return nil // its ticks carry on
	}
	return animTick()
}

// animTick asks for the next frame of the animation.
func animTick() tea.Cmd {
	return tea.Tick(frameInterval, func(t time.Time) tea.Msg { return animTickMsg{at: t} })
}

// stepAnimation moves the animation on to the frame for t.
func (m *Model) stepAnimation(t time.Time) tea.Cmd {
	if m.anim == nil {
		return nil
	}
	m.anim.now = t
	if m.anim.done() {
		m.anim = nil
		return nil
	}
	return animTick()
}

// drawnFrame returns where window w is drawn right now.
func (m Model) drawnFrame(w domain.WindowView) domain.Rect {
	if m.anim == nil {
		return w.Frame
	}
	return m.anim.frameOf(w.ID, w.Frame)
}

// drawnView returns w as it is drawn right now, with its content inside
// the frame wherever that is.
func (m Model) drawnView(w domain.WindowView) domain.WindowView {
	if m.anim == nil {
		return w
	}
	w.Frame = m.drawnFrame(w)
	w.Content = w.Frame.Inset(domain.FrameInset)
	return w
}
