package service

import (
	"errors"
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceClosesWindowWhenPaneExits(t *testing.T) {
	h := start(t, newFakeHost())
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	h.svc.Apply(domain.SplitWindow{Dir: domain.Right})
	h.layoutUntil(windows(2))

	h.svc.Apply(domain.CloseWindow{})
	snap := h.layoutUntil(windows(1))
	if len(snap.Windows) != 1 || snap.Focused != 1 {
		t.Fatalf("unexpected snapshot %+v", snap)
	}
	if got := h.host.size(1); got != (domain.Size{W: 78, H: 22}) {
		t.Fatalf("remaining pane should fill the screen, got %+v", got)
	}

	h.svc.Apply(domain.CloseWindow{})
	if ev := h.next(); ev != (domain.SessionEnded{}) {
		t.Fatalf("expected SessionEnded, got %#v", ev)
	}
	if _, ok := <-h.svc.Events(); ok {
		t.Fatal("events should be closed after SessionEnded")
	}
	if err := <-h.errc; err != nil {
		t.Fatalf("Run returned %v", err)
	}
}

func TestServiceReportsFailedSplit(t *testing.T) {
	host := newFakeHost()
	h := start(t, host)
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	h.layout()

	host.mu.Lock()
	host.spawnErr = errors.New("no pty")
	host.mu.Unlock()
	h.svc.Apply(domain.SplitWindow{Dir: domain.Right})

	failed, ok := h.next().(domain.CommandFailed)
	if !ok || !errors.Is(failed.Err, host.spawnErr) {
		t.Fatalf("expected CommandFailed, got %#v", failed)
	}
	// The failed split leaves the layout untouched; force a fresh one.
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	if snap := h.layout(); len(snap.Windows) != 1 || snap.Focused != 1 {
		t.Fatalf("failed window should be removed, got %+v", snap)
	}
}
