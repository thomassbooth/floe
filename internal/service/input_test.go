package service

import (
	"errors"
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceSendsKeysToAChosenWindow(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right}) // window 2 focused
	c.layoutUntil(windows(2))
	c.apply(domain.SendKey{Key: domain.Key{Code: domain.KeyUp}, Window: 1})

	eventually(t, func() bool {
		h.host.mu.Lock()
		defer h.host.mu.Unlock()
		return len(h.host.keys[1]) == 1
	})
	c.apply(domain.SendKey{Key: domain.Key{Code: 'x'}, Window: 99})
	if failed := c.failure(); !errors.Is(failed.Err, domain.ErrUnknownWindow) {
		t.Fatalf("got %v", failed.Err)
	}
}

func TestServiceSendsMouseEventsToAWindow(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))
	ev := domain.MouseEvent{Action: domain.MousePress, Button: domain.ButtonLeft, X: 3, Y: 4}
	c.apply(domain.SendMouse{Window: 1, Event: ev})

	eventually(t, func() bool {
		h.host.mu.Lock()
		defer h.host.mu.Unlock()
		return len(h.host.mice[1]) == 1 && h.host.mice[1][0] == ev
	})
	c.apply(domain.SendMouse{Window: 99, Event: ev})
	if failed := c.failure(); !errors.Is(failed.Err, domain.ErrUnknownWindow) {
		t.Fatalf("got %v", failed.Err)
	}
}

func TestServiceRoutesInputToFocusedPane(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Down})
	c.apply(domain.FocusDirection{Dir: domain.Up})
	c.apply(domain.SendKey{Key: domain.Key{Code: 'x', Text: "x"}})
	c.apply(domain.Paste{Text: "hello"})

	eventually(t, func() bool {
		h.host.mu.Lock()
		defer h.host.mu.Unlock()
		return len(h.host.pastes[1]) == 1
	})
	h.host.mu.Lock()
	defer h.host.mu.Unlock()
	if len(h.host.keys[1]) != 1 || len(h.host.keys[2]) != 0 {
		t.Fatalf("keys went to the wrong pane: %v", h.host.keys)
	}
	if h.host.pastes[1][0] != "hello" || len(h.host.pastes[2]) != 0 {
		t.Fatalf("paste went to the wrong pane: %v", h.host.pastes)
	}
}
