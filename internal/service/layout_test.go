package service

import (
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceSplitResizesExistingPanes(t *testing.T) {
	h := start(t, newFakeHost())
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	h.layout()

	h.svc.Apply(domain.SplitWindow{Dir: domain.Right})
	snap := h.layout()
	if len(snap.Windows) != 2 || snap.Focused != 2 {
		t.Fatalf("unexpected snapshot %+v", snap)
	}
	if got := h.host.size(1); got != (domain.Size{W: 38, H: 22}) {
		t.Fatalf("pane 1 resized to %+v", got)
	}
	if got := h.host.size(2); got != (domain.Size{W: 38, H: 22}) {
		t.Fatalf("pane 2 spawned at %+v", got)
	}

	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 100, H: 30}})
	h.layout()
	if got := h.host.size(2); got != (domain.Size{W: 48, H: 28}) {
		t.Fatalf("pane 2 resized to %+v", got)
	}
}
