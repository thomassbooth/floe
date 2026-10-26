// Package service runs a session. It is split into small services, one per
// responsibility (lifecycle, windows, focus, layout, input), that share the
// session. A single actor goroutine owns that state and hands each command to
// the service responsible for it, so no locks are needed. Clients send
// commands with Apply and learn about changes from Events.
package service

import (
	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/queue"
)

// Options configures a Service.
type Options struct {
	// Title is the title given to new windows.
	Title string
}

// Service is the entry point to the service layer.
type Service struct {
	host   PaneHost
	inbox  *queue.Unbounded[domain.Command]
	events chan domain.Event

	// Owned by the actor goroutine; see actor.go.
	lifecycle *lifecycleService
	windows   *windowService
	focus     *focusService
	layout    *layoutService
	input     *inputService
	dirty     bool
}

// New returns a service that runs panes on host. Call Run to start it.
func New(host PaneHost, opts Options) *Service {
	session := domain.NewSession()
	layout := newLayoutService(session, host)
	windows := newWindowService(session, host, layout, opts.Title)
	return &Service{
		host:      host,
		inbox:     queue.New[domain.Command](),
		events:    make(chan domain.Event, 16),
		lifecycle: newLifecycleService(session, host, windows),
		windows:   windows,
		focus:     newFocusService(session),
		layout:    layout,
		input:     newInputService(session, host),
	}
}

// Apply queues a command. It never blocks.
func (s *Service) Apply(cmd domain.Command) {
	s.inbox.Push(cmd)
}

// Events receives everything the session announces. It is closed after
// SessionEnded.
func (s *Service) Events() <-chan domain.Event {
	return s.events
}

// Changed receives a signal when any pane's screen changes.
func (s *Service) Changed() <-chan struct{} {
	return s.host.Changed()
}

// Grid copies a pane's current screen. It is safe to call from any
// goroutine.
func (s *Service) Grid(id domain.PaneID) (domain.Grid, bool) {
	return s.host.Grid(id)
}
