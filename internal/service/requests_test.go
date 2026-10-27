package service

import (
	"errors"
	"testing"
	"time"

	"github.com/thomassbooth/floe/internal/domain"
)

func TestRequestAnswersWithTheResult(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()

	if r := result(t, c.c.Request(domain.RenameSession{Name: "work"})); r.Err != nil {
		t.Fatalf("rename failed: %v", r.Err)
	}
	c.layoutUntil(named("work"))
}

func TestRequestFailureGoesToTheCallerNotTheEvents(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.apply(domain.CreateSession{Name: "other"})
	c.layoutUntil(named("other"))

	r := result(t, c.c.Request(domain.RenameSession{Name: "1"}))
	if !errors.Is(r.Err, domain.ErrSessionExists) {
		t.Fatalf("got %v, want %v", r.Err, domain.ErrSessionExists)
	}
	// A command applied after it fails as an event, so if the request's
	// failure had also been sent as one, it would come first.
	c.apply(domain.RenameSession{Name: "1"})
	if ev := c.failure(); ev.Command != (domain.RenameSession{Name: "1"}) || !errors.Is(ev.Err, domain.ErrSessionExists) {
		t.Fatalf("unexpected failure event %#v", ev)
	}
	select {
	case ev := <-c.c.Events():
		if f, ok := ev.(domain.CommandFailed); ok {
			t.Fatalf("one failure was reported twice: %#v", f)
		}
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRequestsKeepTheirPlaceAmongAppliedCommands(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()

	c.apply(domain.CreateSession{Name: "work"})
	r := result(t, c.c.Request(domain.CreateSession{Name: "work"}))
	if !errors.Is(r.Err, domain.ErrSessionExists) {
		t.Fatalf("the request should see the session created just before it, got %v", r.Err)
	}
}

func TestRequestFromAClosedClientIsAnswered(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	c.c.Close()
	if r := result(t, c.c.Request(domain.Tidy{})); !errors.Is(r.Err, errDisconnected) {
		t.Fatalf("got %v, want %v", r.Err, errDisconnected)
	}
}

func TestRequestsAreAnsweredWhenTheServiceStops(t *testing.T) {
	h := start(t, newFakeHost())
	c := h.open()
	h.cancel()
	if err := h.stopped(); err == nil {
		t.Fatal("cancelling should stop the service with the context's error")
	}
	if r := result(t, c.c.Request(domain.Tidy{})); !errors.Is(r.Err, errStopped) {
		t.Fatalf("got %v, want %v", r.Err, errStopped)
	}
}
