package service

import (
	"testing"
	"time"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func TestServiceSizesSessionForMostRecentlyActiveClient(t *testing.T) {
	h := start(t, newFakeHost())
	small := h.open() // 80x24
	if got := h.host.size(1); got != (domain.Size{W: 78, H: 22}) {
		t.Fatalf("pane at %+v", got)
	}

	big := h.connect()
	big.apply(domain.SetArea{Size: domain.Size{W: 120, H: 40}})
	big.apply(domain.AttachSession{})
	big.layoutUntil(func(s domain.Snapshot) bool { return s.Area.W == 120 })
	if got := h.host.size(1); got != (domain.Size{W: 118, H: 38}) {
		t.Fatalf("the newest client's size should win, got %+v", got)
	}

	// Typing in the small client makes it the most recently active.
	small.apply(domain.SendKey{Key: domain.Key{Code: 'a', Text: "a"}})
	small.layoutUntil(func(s domain.Snapshot) bool { return s.Area.W == 80 })
	if got := h.host.size(1); got != (domain.Size{W: 78, H: 22}) {
		t.Fatalf("pane at %+v", got)
	}

	// When it leaves, the big client's size takes over again.
	small.c.Close()
	small.detached()
	big.layoutUntil(func(s domain.Snapshot) bool { return s.Area.W == 120 })
	if got := h.host.size(1); got != (domain.Size{W: 118, H: 38}) {
		t.Fatalf("pane at %+v", got)
	}
}

func TestServiceClosingClientKeepsSessionsRunning(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.c.Close()
	c.detached()

	again := h.connect()
	again.apply(domain.AttachSession{})
	if snap := again.layout(); len(snap.Windows) != 1 || snap.Windows[0].Pane != 1 {
		t.Fatalf("the session should still be there, got %+v", snap)
	}
}

func TestServiceSignalsOutputOnlyToViewers(t *testing.T) {
	h := start(t, newFakeHost())
	a := h.open() // session 1, pane 1
	b := h.connect()
	b.apply(domain.SetArea{Size: domain.Size{W: 80, H: 24}})
	b.apply(domain.CreateSession{Name: "other"}) // pane 2
	b.layoutUntil(named("other"))

	h.host.output(1)
	select {
	case <-a.c.Changed():
	case <-time.After(2 * time.Second):
		t.Fatal("the viewer of pane 1 should be told")
	}
	select {
	case <-b.c.Changed():
		t.Fatal("a client in another session should not be told")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestServiceTellsEveryClientAboutNewSessions(t *testing.T) {
	h := start(t, newFakeHost())
	a := h.open()
	b := h.open()

	a.apply(domain.CreateSession{Name: "work"})
	a.layoutUntil(named("work"))
	for len(b.sessions.Sessions) != 2 {
		b.next()
	}
	if b.sessions.Current == a.sessions.Current {
		t.Fatal("b should still be on session 1")
	}
}
