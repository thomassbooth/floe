// Package service runs the multiplexer's sessions for its clients. It is
// split into small services, one per responsibility (clients, sessions,
// lifecycle, windows, focus, layout, input), that share the domain server.
// A single actor goroutine owns that state and hands each request to the
// service responsible for it, so no locks are needed. Clients connect with
// Connect, send commands with Client.Apply and learn about changes from
// Client.Events.
package service

import (
	"sync/atomic"
	"time"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/queue"
)

// Options configures a Service.
type Options struct {
	// Title is the title given to new windows.
	Title string
	// Now tells the time; time.Now when nil. Tests set it to control time.
	Now func() time.Time
	// Store keeps sessions between runs; nil keeps nothing.
	Store SessionStore
}

// Service is the entry point to the service layer.
type Service struct {
	host       PaneHost
	inbox      *queue.Unbounded[request]
	lastClient atomic.Int64

	// Owned by the actor goroutine; see actor.go.
	server    *domain.Server
	clients   *clientService
	sessions  *sessionService
	lifecycle *lifecycleService
	persist   *persistenceService
	windows   *windowService
	focus     *focusService
	layout    *layoutService
	input     *inputService
	// dirty lists sessions whose layout must be republished.
	dirty map[domain.SessionID]bool
	// listChanged is set when every client needs a new session list.
	listChanged bool
}

// New returns a service that runs panes on host. Call Run to start it.
func New(host PaneHost, opts Options) *Service {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	server := domain.NewServer()
	layout := newLayoutService(host, opts.Now)
	windows := newWindowService(server, host, layout, opts.Title)
	sessions := newSessionService(server, host, windows)
	return &Service{
		host:      host,
		inbox:     queue.New[request](),
		server:    server,
		clients:   newClientService(server),
		sessions:  sessions,
		lifecycle: newLifecycleService(server, sessions),
		persist:   newPersistenceService(opts.Store, host, server, sessions, layout, opts.Title),
		windows:   windows,
		focus:     newFocusService(),
		layout:    layout,
		input:     newInputService(host),
		dirty:     make(map[domain.SessionID]bool),
	}
}

// Connect registers a new client. It starts unattached: send
// AttachSession or CreateSession to show a session, and SetArea to say how
// big the client's screen is.
func (s *Service) Connect() *Client {
	c := newClient(domain.ClientID(s.lastClient.Add(1)), s)
	s.inbox.Push(request{kind: requestConnect, client: c.id, handle: c})
	return c
}
