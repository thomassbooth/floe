package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// fakeStore keeps saved sessions in memory.
type fakeStore struct {
	mu      sync.Mutex
	loaded  []domain.SavedSession
	loadErr error
	saves   [][]domain.SavedSession
}

func (s *fakeStore) Load() ([]domain.SavedSession, error) { return s.loaded, s.loadErr }

func (s *fakeStore) Save(sessions []domain.SavedSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves = append(s.saves, sessions)
	return nil
}

func (s *fakeStore) last() ([]domain.SavedSession, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.saves) == 0 {
		return nil, 0
	}
	return s.saves[len(s.saves)-1], len(s.saves)
}

func startWithStore(t *testing.T, host *fakeHost, store *fakeStore) *harness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h := &harness{t: t, host: host, clock: &clock{now: time.Now()}, cancel: cancel, errc: make(chan error, 1)}
	h.svc = New(host, Options{Title: "sh", Store: store})
	go func() { h.errc <- h.svc.Run(ctx) }()
	return h
}

func TestServiceRestoresSavedSessions(t *testing.T) {
	store := &fakeStore{loaded: []domain.SavedSession{{
		Name: "work",
		Area: domain.Rect{W: 80, H: 24},
		Windows: []domain.SavedWindow{
			{ID: 1, Title: "sh", Frame: domain.Rect{W: 40, H: 24}, Dir: "/home/me"},
			{ID: 3, Title: "vim", Frame: domain.Rect{X: 40, W: 40, H: 24}, Dir: "/src", Focused: true},
		},
	}}}
	h := startWithStore(t, newFakeHost(), store)

	c := h.connect()
	c.apply(domain.AttachSession{})
	snap := c.layoutUntil(windows(2))
	if snap.Name != "work" || snap.Focused != 3 {
		t.Fatalf("restored %+v", snap)
	}
	h.host.mu.Lock()
	defer h.host.mu.Unlock()
	dirs := map[string]bool{}
	for _, d := range h.host.spawnDirs {
		dirs[d] = true
	}
	if !dirs["/home/me"] || !dirs["/src"] {
		t.Fatalf("shells should start in the saved directories, got %v", h.host.spawnDirs)
	}
	for id, size := range h.host.sizes {
		if size != (domain.Size{W: 38, H: 22}) {
			t.Fatalf("pane %d started at %+v", id, size)
		}
	}
}

func TestServiceSavesChangesAndOnShutdown(t *testing.T) {
	store := &fakeStore{}
	h := startWithStore(t, newFakeHost(), store)
	c := h.open()
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))

	eventually3s(t, func() bool {
		saved, _ := store.last()
		return len(saved) == 1 && len(saved[0].Windows) == 2
	})

	h.cancel()
	h.stopped()
	saved, _ := store.last()
	if len(saved) != 1 || saved[0].Windows[0].Dir == "" {
		t.Fatalf("the last save should know each window's directory: %+v", saved)
	}
}

func TestServiceSavesNothingAfterTheLastSessionIsKilled(t *testing.T) {
	store := &fakeStore{}
	h := startWithStore(t, newFakeHost(), store)
	c := h.open()
	c.apply(domain.KillSession{})
	c.detached()
	h.stopped()
	if saved, n := store.last(); n == 0 || len(saved) != 0 {
		t.Fatalf("expected an empty save, got %d saves, last %+v", n, saved)
	}
}

func TestServiceReportsRestoreErrorsToClients(t *testing.T) {
	store := &fakeStore{loadErr: errors.New("corrupt")}
	h := startWithStore(t, newFakeHost(), store)
	c := h.connect()
	if failed := c.failure(); !errors.Is(failed.Err, store.loadErr) {
		t.Fatalf("got %v", failed.Err)
	}
}

func eventually3s(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
