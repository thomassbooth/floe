package service

import (
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceSplitResizesExistingPanes(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()

	c.apply(domain.SplitWindow{Dir: domain.Right})
	snap := c.layoutUntil(windows(2))
	if snap.Focused != 2 {
		t.Fatalf("unexpected snapshot %+v", snap)
	}
	if got := h.host.size(1); got != (domain.Size{W: 38, H: 22}) {
		t.Fatalf("pane 1 resized to %+v", got)
	}
	if got := h.host.size(2); got != (domain.Size{W: 38, H: 22}) {
		t.Fatalf("pane 2 spawned at %+v", got)
	}

	c.apply(domain.SetArea{Size: domain.Size{W: 100, H: 30}})
	c.layoutUntil(func(s domain.Snapshot) bool { return s.Area.W == 100 })
	if got := h.host.size(2); got != (domain.Size{W: 48, H: 28}) {
		t.Fatalf("pane 2 resized to %+v", got)
	}
}

func TestServiceKeyboardResizeShrinksFocusedWindow(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))

	c.apply(domain.ResizeWindow{Dir: domain.Left, Amount: 10})
	snap := c.layout()
	if w := snap.Windows[1].Frame.W; w != 30 {
		t.Fatalf("window 2 width = %d, want 30", w)
	}
	if got := h.host.size(2); got != (domain.Size{W: 28, H: 22}) {
		t.Fatalf("pane 2 resized to %+v", got)
	}
}

func TestServicePlacesAndResizesWindowsByEdge(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()

	c.apply(domain.PlaceWindow{ID: 1, Frame: domain.Rect{X: 10, Y: 2, W: 40, H: 12}})
	if snap := c.layout(); snap.Windows[0].Frame != (domain.Rect{X: 10, Y: 2, W: 40, H: 12}) {
		t.Fatalf("frame = %+v", snap.Windows[0].Frame)
	}
	if got := h.host.size(1); got != (domain.Size{W: 38, H: 10}) {
		t.Fatalf("pane resized to %+v", got)
	}

	c.apply(domain.ResizeWindowEdge{ID: 1, Edge: domain.Right, Position: 70})
	if snap := c.layout(); snap.Windows[0].Frame.W != 60 {
		t.Fatalf("frame = %+v", snap.Windows[0].Frame)
	}
	if got := h.host.size(1); got != (domain.Size{W: 58, H: 10}) {
		t.Fatalf("pane resized to %+v", got)
	}
}

func TestServiceTogglesMaximize(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))

	c.apply(domain.ToggleMaximize{})
	snap := c.layout()
	if w := snap.Windows[1]; !w.Maximized || w.Frame.W != 80 {
		t.Fatalf("window = %+v", w)
	}
	if got := h.host.size(2); got != (domain.Size{W: 78, H: 22}) {
		t.Fatalf("pane resized to %+v", got)
	}

	c.apply(domain.ToggleMaximize{ID: 2})
	if w := c.layout().Windows[1]; w.Maximized || w.Frame.W != 40 {
		t.Fatalf("window = %+v", w)
	}
}

func TestServiceSnapsAndTidies(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.NewWindow{Frame: domain.Rect{X: 20, Y: 6, W: 40, H: 12}})
	c.layoutUntil(windows(2))

	c.apply(domain.SnapWindow{ID: 2, Zone: domain.ZoneRight})
	snap := c.layout()
	if w := snap.Windows[1]; w.Frame != (domain.Rect{X: 40, W: 40, H: 24}) || w.Restore.W != 40 {
		t.Fatalf("snapped window = %+v", w)
	}
	if got := h.host.size(2); got != (domain.Size{W: 38, H: 22}) {
		t.Fatalf("pane resized to %+v", got)
	}

	c.apply(domain.Tidy{})
	snap = c.layout()
	if snap.Windows[0].Frame != (domain.Rect{W: 40, H: 24}) {
		t.Fatalf("window 1 should take the left half, got %+v", snap.Windows[0].Frame)
	}
	if got := h.host.size(1); got != (domain.Size{W: 38, H: 22}) {
		t.Fatalf("pane 1 resized to %+v", got)
	}
}
