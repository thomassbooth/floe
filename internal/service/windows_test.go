package service

import (
	"errors"
	"testing"
	"time"

	"github.com/thomassbooth/floe/internal/domain"
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

func TestServiceOpensNewWindowsInTheFocusedWindowsDirectory(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))
	c.apply(domain.NewWindow{Frame: domain.Rect{X: 20, Y: 6, W: 40, H: 12}})
	c.layoutUntil(windows(3))

	h.host.mu.Lock()
	defer h.host.mu.Unlock()
	if d := h.host.spawnDirs[1]; d != "" {
		t.Fatalf("the first window has nothing to inherit, got %q", d)
	}
	if d := h.host.spawnDirs[2]; d != "/dir/of/1" {
		t.Fatalf("the split should start where window 1 is, got %q", d)
	}
	if d := h.host.spawnDirs[3]; d != "/dir/of/2" {
		t.Fatalf("the new window should start where window 2 is, got %q", d)
	}
}

func TestServiceUpdatesWindowTitles(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	h.host.retitle(1, "vim")
	snap := c.layoutUntil(func(s domain.Snapshot) bool { return s.Windows[0].Title == "vim" })
	if snap.Windows[0].Title != "vim" {
		t.Fatalf("title = %q", snap.Windows[0].Title)
	}
	h.host.retitle(99, "ghost") // unknown panes are ignored
}

func TestServicePassesClipboardCopiesToViewers(t *testing.T) {
	h := start(t, newFakeHost())
	a := h.open() // session 1, pane 1
	b := h.connect()
	b.apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	b.apply(domain.CreateSession{Name: "other"})
	b.layoutUntil(named("other"))

	h.host.notify(domain.PaneNotice{Pane: 1, Kind: domain.NoticeClipboard, Text: "copied"})
	for {
		if ev, ok := a.next().(domain.ClipboardSet); ok {
			if ev.Text != "copied" {
				t.Fatalf("clipboard = %q", ev.Text)
			}
			break
		}
	}
	h.host.retitle(2, "marker") // b only hears about its own session
	for {
		switch ev := b.next().(type) {
		case domain.ClipboardSet:
			t.Fatal("a client in another session must not get the copy")
		case domain.LayoutChanged:
			if ev.Snapshot.Windows[0].Title == "marker" {
				return
			}
		}
	}
}

// handled waits until the service has dealt with pane output, then retitles
// window 1 and returns the layout that shows it, so the test can look at
// the effect of that output.
func handled(t *testing.T, h *harness, c *testClient, title string) domain.Snapshot {
	t.Helper()
	select {
	case <-c.c.Changed():
	case <-time.After(2 * time.Second):
		t.Fatal("the output was never handled")
	}
	h.host.retitle(1, title)
	return c.layoutUntil(func(s domain.Snapshot) bool { return s.Windows[0].Title == title })
}

func busy(id domain.WindowID, want bool) func(domain.Snapshot) bool {
	return func(s domain.Snapshot) bool {
		for _, w := range s.Windows {
			if w.ID == id {
				return w.Busy == want
			}
		}
		return false
	}
}

func TestServiceMarksPanesThatKeepPrintingBusyUntilQuiet(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right}) // resizes window 1
	c.layoutUntil(windows(2))

	// Window 1's shell redraws to fit its new size: not work.
	h.host.output(1)
	handled(t, h, c, "redrawn")
	h.clock.advance(time.Second)

	// One burst of output, like a prompt, is not work either.
	h.host.output(1)
	if snap := handled(t, h, c, "burst"); snap.Windows[0].Busy {
		t.Fatal("a single burst is not work")
	}

	// Output that keeps coming is, whether the window has focus or not.
	h.clock.advance(busyAfter)
	h.host.output(1)
	c.layoutUntil(busy(1, true))
	h.host.output(2)
	handled(t, h, c, "started")
	h.clock.advance(busyAfter)
	h.host.output(2)
	c.layoutUntil(busy(2, true))

	// Window 1 prints again, so it outlasts window 2.
	h.clock.advance(busyAfter)
	h.host.output(1)
	handled(t, h, c, "printing")
	h.clock.advance(busyQuiet - busyAfter)
	snap := c.layoutUntil(busy(2, false))
	if !snap.Windows[0].Busy {
		t.Fatal("a pane that kept printing is still busy")
	}
	h.clock.advance(busyAfter)
	c.layoutUntil(busy(1, false))
}

func TestServiceDoesNotTakeEchoesAsWork(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()

	c.apply(domain.SendKey{Key: domain.Key{Code: 'a', Text: "a"}})
	eventually(t, func() bool {
		h.host.mu.Lock()
		defer h.host.mu.Unlock()
		return len(h.host.keys[1]) == 1
	})
	h.host.output(1)
	handled(t, h, c, "echoed")

	// Output after the echo starts afresh, as though the echo never was.
	h.clock.advance(echoQuiet)
	h.host.output(1)
	if snap := handled(t, h, c, "started"); snap.Windows[0].Busy {
		t.Fatal("the echo of a key is not work")
	}
	h.clock.advance(busyAfter)
	h.host.output(1)
	c.layoutUntil(busy(1, true))
}

func TestServiceMarksBellsOnUnfocusedWindowsUntilFocused(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))

	h.host.notify(domain.PaneNotice{Pane: 2, Kind: domain.NoticeBell})
	h.host.notify(domain.PaneNotice{Pane: 1, Kind: domain.NoticeBell})
	snap := c.layoutUntil(func(s domain.Snapshot) bool { return s.Windows[0].Bell })
	if snap.Windows[1].Bell {
		t.Fatal("the focused window is never marked")
	}

	c.apply(domain.FocusWindow{ID: 1})
	c.layoutUntil(func(s domain.Snapshot) bool {
		w := s.Windows[len(s.Windows)-1]
		return w.ID == 1 && !w.Bell
	})
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
