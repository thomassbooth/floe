package service

import (
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceRoutesInputToFocusedPane(t *testing.T) {
	h := start(t, newFakeHost())
	h.svc.Apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	h.svc.Apply(domain.SplitWindow{Dir: domain.Down})
	h.svc.Apply(domain.FocusDirection{Dir: domain.Up})
	h.svc.Apply(domain.SendKey{Key: domain.Key{Code: 'x', Text: "x"}})
	h.svc.Apply(domain.Paste{Text: "hello"})

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
