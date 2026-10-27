package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/queue"
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
	dirty    []domain.PaneID

	mice      map[domain.PaneID][]domain.MouseEvent
	spawnDirs map[domain.PaneID]string
	notices   *queue.Unbounded[domain.PaneNotice]
}

func newFakeHost() *fakeHost {
	return &fakeHost{
		sizes:   map[domain.PaneID]domain.Size{},
		keys:    map[domain.PaneID][]domain.Key{},
		pastes:  map[domain.PaneID][]string{},
		exited:  make(chan domain.PaneID, 16),
		changed: make(chan struct{}, 1),

		mice:      map[domain.PaneID][]domain.MouseEvent{},
		spawnDirs: map[domain.PaneID]string{},
		notices:   queue.New[domain.PaneNotice](),
	}
}

func (h *fakeHost) Spawn(id domain.PaneID, size domain.Size, dir string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.spawnErr != nil {
		return h.spawnErr
	}
	h.sizes[id] = size
	h.spawnDirs[id] = dir
	return nil
}

func (h *fakeHost) WorkingDir(id domain.PaneID) (string, bool) {
	return fmt.Sprintf("/dir/of/%d", id), true
}

func (h *fakeHost) Notices() <-chan struct{}         { return h.notices.Ready() }
func (h *fakeHost) TakeNotices() []domain.PaneNotice { return h.notices.Drain() }
func (h *fakeHost) notify(n domain.PaneNotice)       { h.notices.Push(n) }

// retitle pretends a pane's title changed.
func (h *fakeHost) retitle(id domain.PaneID, title string) {
	h.notify(domain.PaneNotice{Pane: id, Kind: domain.NoticeTitle, Text: title})
}

func (h *fakeHost) Resize(id domain.PaneID, size domain.Size) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.sizes[id]; ok {
		h.sizes[id] = size
	}
}

func (h *fakeHost) SendKey(id domain.PaneID, key domain.Key) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.keys[id] = append(h.keys[id], key)
}

func (h *fakeHost) SendMouse(id domain.PaneID, ev domain.MouseEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.mice[id] = append(h.mice[id], ev)
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

func (h *fakeHost) Grid(domain.PaneID, int) (domain.Grid, bool)  { return domain.Grid{}, false }
func (h *fakeHost) State(domain.PaneID) (domain.PaneState, bool) { return domain.PaneState{}, false }
func (h *fakeHost) Exited() <-chan domain.PaneID                 { return h.exited }
func (h *fakeHost) Changed() <-chan struct{}                     { return h.changed }

func (h *fakeHost) TakeChanged() []domain.PaneID {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.dirty
	h.dirty = nil
	return out
}

// output pretends a pane printed something.
func (h *fakeHost) output(id domain.PaneID) {
	h.mu.Lock()
	h.dirty = append(h.dirty, id)
	h.mu.Unlock()
	select {
	case h.changed <- struct{}{}:
	default:
	}
}

func (h *fakeHost) running(id domain.PaneID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.sizes[id]
	return ok
}

func (h *fakeHost) size(id domain.PaneID) domain.Size {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sizes[id]
}

type harness struct {
	t      *testing.T
	host   *fakeHost
	svc    *Service
	cancel context.CancelFunc
	errc   chan error
}

func start(t *testing.T, host *fakeHost) *harness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h := &harness{t: t, host: host, svc: New(host, Options{Title: "sh"}), cancel: cancel, errc: make(chan error, 1)}
	go func() { h.errc <- h.svc.Run(ctx) }()
	return h
}

// stopped waits for Run to return and gives its error.
func (h *harness) stopped() error {
	h.t.Helper()
	select {
	case err := <-h.errc:
		return err
	case <-time.After(2 * time.Second):
		h.t.Fatal("service did not stop")
	}
	return nil
}

// testClient wraps a Client with helpers that wait for events.
type testClient struct {
	t *testing.T
	c *Client
	// sessions is the latest session list the client was sent.
	sessions domain.SessionsChanged
}

func (h *harness) connect() *testClient {
	return &testClient{t: h.t, c: h.svc.Connect()}
}

// open connects a client with an 80x24 screen to the most recent session,
// creating one if needed, and waits for a window to show.
func (h *harness) open() *testClient {
	h.t.Helper()
	c := h.connect()
	c.apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	c.apply(domain.AttachSession{})
	c.layoutUntil(func(s domain.Snapshot) bool { return len(s.Windows) > 0 })
	return c
}

func (c *testClient) apply(cmd domain.Command) { c.c.Apply(cmd) }

// next waits for the client's next event.
func (c *testClient) next() domain.Event {
	c.t.Helper()
	select {
	case ev, ok := <-c.c.Events():
		if !ok {
			c.t.Fatal("events closed")
		}
		if sc, ok := ev.(domain.SessionsChanged); ok {
			c.sessions = sc
		}
		return ev
	case <-time.After(2 * time.Second):
		c.t.Fatal("timed out waiting for an event")
	}
	return nil
}

// layout waits for the next layout, failing on errors along the way.
func (c *testClient) layout() domain.Snapshot {
	c.t.Helper()
	for {
		switch ev := c.next().(type) {
		case domain.LayoutChanged:
			return ev.Snapshot
		case domain.CommandFailed, domain.Detached:
			c.t.Fatalf("expected a layout, got %#v", ev)
		}
	}
}

// layoutUntil waits for a layout matching ok. Commands applied together may
// be handled in one batch or several, so intermediate layouts are skipped.
func (c *testClient) layoutUntil(ok func(domain.Snapshot) bool) domain.Snapshot {
	c.t.Helper()
	for {
		if snap := c.layout(); ok(snap) {
			return snap
		}
	}
}

// failure waits for the next CommandFailed.
func (c *testClient) failure() domain.CommandFailed {
	c.t.Helper()
	for {
		switch ev := c.next().(type) {
		case domain.CommandFailed:
			return ev
		case domain.Detached:
			c.t.Fatalf("expected a failure, got %#v", ev)
		}
	}
}

// detached waits for Detached and checks that no events follow it.
func (c *testClient) detached() domain.Detached {
	c.t.Helper()
	for {
		if ev, ok := c.next().(domain.Detached); ok {
			if _, open := <-c.c.Events(); open {
				c.t.Fatal("events should be closed after Detached")
			}
			return ev
		}
	}
}

func windows(n int) func(domain.Snapshot) bool {
	return func(s domain.Snapshot) bool { return len(s.Windows) == n }
}

func named(name string) func(domain.Snapshot) bool {
	return func(s domain.Snapshot) bool { return s.Name == name && len(s.Windows) > 0 }
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
