package service

import (
	"errors"
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceClosesWindowWhenPaneExits(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))

	c.apply(domain.CloseWindow{})
	snap := c.layoutUntil(windows(1))
	if snap.Focused != 1 {
		t.Fatalf("unexpected snapshot %+v", snap)
	}
	if got := h.host.size(1); got != (domain.Size{W: 78, H: 22}) {
		t.Fatalf("remaining pane should fill the screen, got %+v", got)
	}

	// The last window closing ends the only session, so the client detaches.
	c.apply(domain.CloseWindow{})
	if ev := c.detached(); ev.Err != nil {
		t.Fatalf("unexpected error %v", ev.Err)
	}
}

func TestServiceReportsFailedSplit(t *testing.T) {
	host := newFakeHost()
	h := start(t, host)
	c := h.open()

	host.mu.Lock()
	host.spawnErr = errors.New("no pty")
	host.mu.Unlock()
	c.apply(domain.SplitWindow{Dir: domain.Right})

	if failed := c.failure(); !errors.Is(failed.Err, host.spawnErr) {
		t.Fatalf("expected the spawn error, got %v", failed.Err)
	}
	// The failed split leaves the layout untouched; force a fresh one.
	c.apply(domain.SetArea{Size: domain.Size{W: 60, H: 20}})
	snap := c.layout()
	if len(snap.Windows) != 1 || snap.Focused != 1 {
		t.Fatalf("failed window should be removed, got %+v", snap)
	}
	if snap.Windows[0].Frame != (domain.Rect{W: 60, H: 20}) {
		t.Fatalf("the split window should get its space back, got %+v", snap.Windows[0].Frame)
	}
}

func TestServiceOpensWindowAtFrame(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.NewWindow{Frame: domain.Rect{X: 20, Y: 6, W: 40, H: 12}})
	snap := c.layoutUntil(windows(2))
	if w := snap.Windows[1]; w.ID != 2 || w.Frame != (domain.Rect{X: 20, Y: 6, W: 40, H: 12}) || !w.Focused {
		t.Fatalf("new window = %+v", w)
	}
	if snap.Windows[0].Frame.W != 80 {
		t.Fatal("the existing window should not be split")
	}
	if got := h.host.size(2); got != (domain.Size{W: 38, H: 10}) {
		t.Fatalf("pane started at %+v", got)
	}
}

func TestServiceClosesWindowByID(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))

	c.apply(domain.CloseWindow{ID: 1})
	snap := c.layoutUntil(windows(1))
	if snap.Windows[0].ID != 2 || h.host.running(1) {
		t.Fatalf("window 1 should be closed, got %+v", snap)
	}

	c.apply(domain.CloseWindow{ID: 99})
	if failed := c.failure(); !errors.Is(failed.Err, domain.ErrUnknownWindow) {
		t.Fatalf("got %v", failed.Err)
	}
}

func TestServiceRejectsWindowCommandsWhenNotAttached(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.connect()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	if failed := c.failure(); !errors.Is(failed.Err, errNotAttached) {
		t.Fatalf("got %v", failed.Err)
	}
}
