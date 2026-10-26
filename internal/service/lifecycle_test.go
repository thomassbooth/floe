package service

import (
	"errors"
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceStartsFirstWindowOnceAreaIsKnown(t *testing.T) {
	h := start(t, newFakeHost())
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})

	snap := h.layout()
	if len(snap.Windows) != 1 || snap.Focused != 1 {
		t.Fatalf("unexpected snapshot %+v", snap)
	}
	if got := h.host.size(1); got != (domain.Size{W: 78, H: 22}) {
		t.Fatalf("pane spawned at %+v", got)
	}
}

func TestServiceKillSessionEndsEverything(t *testing.T) {
	h := start(t, newFakeHost())
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	h.svc.Apply(domain.SplitWindow{Dir: domain.Right})
	h.svc.Apply(domain.SplitWindow{Dir: domain.Down})
	h.layoutUntil(windows(3))

	h.svc.Apply(domain.KillSession{})
	for {
		ev := h.next()
		if _, ok := ev.(domain.SessionEnded); ok {
			break
		}
	}
	if err := <-h.errc; err != nil {
		t.Fatalf("Run returned %v", err)
	}
}

func TestServiceEndsWhenFirstPaneCannotStart(t *testing.T) {
	host := newFakeHost()
	host.spawnErr = errors.New("no shell")
	h := start(t, host)
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})

	ended, ok := h.next().(domain.SessionEnded)
	if !ok || !errors.Is(ended.Err, host.spawnErr) {
		t.Fatalf("expected SessionEnded with error, got %#v", ended)
	}
	if err := <-h.errc; !errors.Is(err, host.spawnErr) {
		t.Fatalf("Run returned %v", err)
	}
}
