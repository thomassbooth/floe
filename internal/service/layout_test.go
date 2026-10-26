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

func TestServiceResizeMovesDivider(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))

	c.apply(domain.ResizeWindow{Dir: domain.Left, Amount: 10})
	snap := c.layout()
	if w := snap.Windows[1].Frame.W; w != 50 {
		t.Fatalf("window 2 width = %d, want 50", w)
	}
	if got := h.host.size(2); got != (domain.Size{W: 48, H: 22}) {
		t.Fatalf("pane 2 resized to %+v", got)
	}
}
