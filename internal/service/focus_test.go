package service

import (
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceMovesFocus(t *testing.T) {
	h := start(t, newFakeHost())
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	h.svc.Apply(domain.SplitWindow{Dir: domain.Right})
	h.layoutUntil(windows(2))

	h.svc.Apply(domain.FocusDirection{Dir: domain.Left})
	if snap := h.layout(); snap.Focused != 1 {
		t.Fatalf("focus = %d, want 1", snap.Focused)
	}

	h.svc.Apply(domain.FocusWindow{ID: 2})
	if snap := h.layout(); snap.Focused != 2 {
		t.Fatalf("focus = %d, want 2", snap.Focused)
	}
}

func TestServiceSkipsLayoutWhenFocusCannotMove(t *testing.T) {
	h := start(t, newFakeHost())
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	h.layout()

	// Nothing to the left: no new layout. The resize afterwards proves the
	// next event belongs to it rather than to the focus command.
	h.svc.Apply(domain.FocusDirection{Dir: domain.Left})
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 60, H: 20}})
	if snap := h.layout(); snap.Area.W != 60 {
		t.Fatalf("expected only the resize layout, got %+v", snap)
	}
}
