package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// fakeHost records what the service asks of it. Kill reports the pane as
// exited, like a real process ending.
type fakeHost struct {
	mu       sync.Mutex
	sizes    map[domain.PaneID]domain.Size
	keys     map[domain.PaneID][]domain.Key
	pastes   map[domain.PaneID][]string
	spawnErr error
	exited   chan domain.PaneID
	changed  chan struct{}
}

func newFakeHost() *fakeHost {
	return &fakeHost{
		sizes:   map[domain.PaneID]domain.Size{},
		keys:    map[domain.PaneID][]domain.Key{},
		pastes:  map[domain.PaneID][]string{},
		exited:  make(chan domain.PaneID, 16),
		changed: make(chan struct{}, 1),
	}
}

func (h *fakeHost) Spawn(id domain.PaneID, size domain.Size) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.spawnErr != nil {
		return h.spawnErr
	}
	h.sizes[id] = size
	return nil
}

func (h *fakeHost) Resize(id domain.PaneID, size domain.Size) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sizes[id] = size
}

func (h *fakeHost) SendKey(id domain.PaneID, key domain.Key) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.keys[id] = append(h.keys[id], key)
}

func (h *fakeHost) Paste(id domain.PaneID, text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pastes[id] = append(h.pastes[id], text)
}

func (h *fakeHost) Kill(id domain.PaneID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.sizes[id]; ok {
		delete(h.sizes, id)
		h.exited <- id
	}
}

func (h *fakeHost) Grid(domain.PaneID) (domain.Grid, bool) { return domain.Grid{}, false }
func (h *fakeHost) Exited() <-chan domain.PaneID           { return h.exited }
func (h *fakeHost) Changed() <-chan struct{}               { return h.changed }

func (h *fakeHost) size(id domain.PaneID) domain.Size {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sizes[id]
}

type harness struct {
	t    *testing.T
	host *fakeHost
	svc  *Service
	errc chan error
}

func start(t *testing.T, host *fakeHost) *harness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h := &harness{t: t, host: host, svc: New(host, Options{Title: "sh"}), errc: make(chan error, 1)}
	go func() { h.errc <- h.svc.Run(ctx) }()
	return h
}

// next waits for the next event.
func (h *harness) next() domain.Event {
	h.t.Helper()
	select {
	case ev, ok := <-h.svc.Events():
		if !ok {
			h.t.Fatal("events closed")
		}
		return ev
	case <-time.After(2 * time.Second):
		h.t.Fatal("timed out waiting for an event")
	}
	return nil
}

func (h *harness) layout() domain.Snapshot {
	h.t.Helper()
	ev := h.next()
	lc, ok := ev.(domain.LayoutChanged)
	if !ok {
		h.t.Fatalf("expected LayoutChanged, got %#v", ev)
	}
	return lc.Snapshot
}

// layoutUntil waits for a layout matching ok. Commands applied together may
// be handled in one batch or several, so intermediate layouts are skipped.
func (h *harness) layoutUntil(ok func(domain.Snapshot) bool) domain.Snapshot {
	h.t.Helper()
	for {
		if snap := h.layout(); ok(snap) {
			return snap
		}
	}
}

func windows(n int) func(domain.Snapshot) bool {
	return func(s domain.Snapshot) bool { return len(s.Windows) == n }
}

func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
