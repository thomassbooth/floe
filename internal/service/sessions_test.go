package service

import (
	"errors"
	"testing"

	"github.com/thomassbooth/floe/internal/domain"
)

func TestServiceCreatesAndSwitchesSessions(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open() // session "1", pane 1

	c.apply(domain.CreateSession{Name: "work"})
	snap := c.layoutUntil(named("work"))
	if snap.Windows[0].Pane != 2 {
		t.Fatalf("new session should get a new pane, got %+v", snap.Windows)
	}
	if got := c.sessions; len(got.Sessions) != 2 || got.Current != snap.Session {
		t.Fatalf("session list = %+v", got)
	}

	c.apply(domain.SendKey{Key: domain.Key{Code: 'a', Text: "a"}})
	c.apply(domain.SwitchSession{ID: 1})
	c.layoutUntil(named("1"))
	c.apply(domain.SendKey{Key: domain.Key{Code: 'b', Text: "b"}})

	eventually(t, func() bool {
		h.host.mu.Lock()
		defer h.host.mu.Unlock()
		return len(h.host.keys[1]) == 1
	})
	h.host.mu.Lock()
	defer h.host.mu.Unlock()
	if h.host.keys[2][0].Code != 'a' || h.host.keys[1][0].Code != 'b' {
		t.Fatalf("keys went to the wrong sessions: %v", h.host.keys)
	}
}

func TestServiceAttachesByName(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.CreateSession{Name: "work"})
	c.layoutUntil(named("work"))

	c.apply(domain.AttachSession{Name: "1"})
	c.layoutUntil(named("1"))

	c.apply(domain.AttachSession{Name: "nope"})
	if failed := c.failure(); !errors.Is(failed.Err, domain.ErrUnknownSession) {
		t.Fatalf("got %v", failed.Err)
	}
	c.apply(domain.CreateSession{Name: "work"})
	if failed := c.failure(); !errors.Is(failed.Err, domain.ErrSessionExists) {
		t.Fatalf("got %v", failed.Err)
	}
}

func TestServiceAttachWithoutNamePicksMostRecentSession(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.CreateSession{Name: "work"})
	c.layoutUntil(named("work"))

	other := h.connect()
	other.apply(domain.AttachSession{})
	if snap := other.layout(); snap.Name != "work" {
		t.Fatalf("attached to %q, want work", snap.Name)
	}
}

func TestServiceRenamesSession(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.RenameSession{Name: "dev"})
	c.layoutUntil(named("dev"))
	if got := c.sessions.Sessions; len(got) != 1 || got[0].Name != "dev" {
		t.Fatalf("session list = %+v", got)
	}

	c.apply(domain.RenameSession{Name: " "})
	if failed := c.failure(); !errors.Is(failed.Err, domain.ErrInvalidName) {
		t.Fatalf("got %v", failed.Err)
	}
}

func TestServiceKillSessionMovesClientToMostRecent(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.CreateSession{Name: "work"})
	c.layoutUntil(named("work"))
	c.apply(domain.SplitWindow{Dir: domain.Right})
	c.layoutUntil(windows(2))

	c.apply(domain.KillSession{})
	c.layoutUntil(named("1"))
	if h.host.running(2) || h.host.running(3) {
		t.Fatal("the killed session's panes should be gone")
	}
	if got := c.sessions.Sessions; len(got) != 1 || got[0].Name != "1" {
		t.Fatalf("session list = %+v", got)
	}
}

func TestServiceKillsAnotherSessionByID(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.CreateSession{Name: "work"})
	c.layoutUntil(named("work"))

	c.apply(domain.KillSession{ID: 1})
	eventually(t, func() bool { return !h.host.running(1) })
	for len(c.sessions.Sessions) != 1 {
		c.next()
	}
	if c.sessions.Sessions[0].Name != "work" {
		t.Fatalf("session list = %+v", c.sessions)
	}
}
