package service

import (
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceMovesFocus(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))

	c.apply(domain.FocusDirection{Dir: domain.Left})
	if snap := c.layout(); snap.Focused != 1 {
		t.Fatalf("focus = %d, want 1", snap.Focused)
	}

	c.apply(domain.FocusWindow{ID: 2})
	if snap := c.layout(); snap.Focused != 2 {
		t.Fatalf("focus = %d, want 2", snap.Focused)
	}
}

func TestServiceSkipsLayoutWhenFocusCannotMove(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()

	// Nothing to the left: no new layout. The resize afterwards proves the
	// next layout belongs to it rather than to the focus command.
	c.apply(domain.FocusDirection{Dir: domain.Left})
	c.apply(domain.SetArea{Size: domain.Size{W: 60, H: 20}})
	if snap := c.layout(); snap.Area.W != 60 {
		t.Fatalf("expected only the resize layout, got %+v", snap)
	}
}
