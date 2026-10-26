package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceStartsFirstWindowOnceAreaIsKnown(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.connect()
	c.apply(domain.AttachSession{})
	if snap := c.layout(); len(snap.Windows) != 0 || snap.Name != "1" {
		t.Fatalf("session should wait for an area, got %+v", snap)
	}

	c.apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	snap := c.layoutUntil(windows(1))
	if snap.Focused != 1 {
		t.Fatalf("unexpected snapshot %+v", snap)
	}
	if got := h.host.size(1); got != (domain.Size{W: 78, H: 22}) {
		t.Fatalf("pane spawned at %+v", got)
	}
}

func TestServiceStopsWhenLastSessionEnds(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))

	c.apply(domain.KillSession{})
	if ev := c.detached(); ev.Err != nil {
		t.Fatalf("unexpected error %v", ev.Err)
	}
	if err := h.stopped(); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	if h.host.running(1) || h.host.running(2) {
		t.Fatal("every pane should be killed")
	}
}

func TestServiceDetachesWithErrorWhenFirstPaneCannotStart(t *testing.T) {
	host := newFakeHost()
	host.spawnErr = errors.New("no shell")
	h := start(t, host)
	c := h.connect()
	c.apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	c.apply(domain.AttachSession{})

	if ev := c.detached(); !errors.Is(ev.Err, host.spawnErr) {
		t.Fatalf("expected the spawn error, got %v", ev.Err)
	}
	if err := h.stopped(); err != nil {
		t.Fatalf("Run returned %v", err)
	}
}

func TestServiceCancelDetachesClientsAndKillsPanes(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()

	h.cancel()
	c.detached()
	if err := h.stopped(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v", err)
	}
	if h.host.running(1) {
		t.Fatal("pane should be killed")
	}
}
