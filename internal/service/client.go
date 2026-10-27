package service

import (
	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/queue"
)

// Client is one connection to the service, such as a terminal showing a
// session. Its methods are safe to call from any goroutine.
type Client struct {
	id      domain.ClientID
	svc     *Service
	queue   *queue.Unbounded[domain.Event]
	events  chan domain.Event
	changed chan struct{}
}

func newClient(id domain.ClientID, svc *Service) *Client {
	c := &Client{
		id:      id,
		svc:     svc,
		queue:   queue.New[domain.Event](),
		events:  make(chan domain.Event),
		changed: make(chan struct{}, 1),
	}
	go c.pump()
	return c
}

// ID returns the client's ID.
func (c *Client) ID() domain.ClientID { return c.id }

// Apply queues a command from this client. It never blocks. If the command
// fails, a CommandFailed event says so.
func (c *Client) Apply(cmd domain.Command) {
	c.svc.inbox.Push(request{kind: requestCommand, client: c.id, cmd: cmd})
}

// Request queues a command like Apply, in order with the commands before
// it, and returns a channel that receives its result once. A failure goes
// only to the channel, not to the events. It never blocks.
func (c *Client) Request(cmd domain.Command) <-chan domain.Result {
	reply := make(chan domain.Result, 1)
	if !c.svc.inbox.Push(request{kind: requestCommand, client: c.id, cmd: cmd, reply: reply}) {
		reply <- domain.Result{Err: errStopped}
	}
	return reply
}

// Events receives everything the server tells this client. It is closed
// after Detached.
func (c *Client) Events() <-chan domain.Event { return c.events }

// Changed receives a signal when a pane in the client's session changes.
// Signals coalesce.
func (c *Client) Changed() <-chan struct{} { return c.changed }

// Grid copies a pane's screen, scrolled scroll lines up into its history.
func (c *Client) Grid(id domain.PaneID, scroll int) (domain.Grid, bool) {
	return c.svc.host.Grid(id, scroll)
}

// State describes a pane's history and input modes.
func (c *Client) State(id domain.PaneID) (domain.PaneState, bool) {
	return c.svc.host.State(id)
}

// Close disconnects the client. Its sessions keep running.
func (c *Client) Close() {
	c.svc.inbox.Push(request{kind: requestDisconnect, client: c.id})
}

// deliver queues an event without blocking the actor.
func (c *Client) deliver(ev domain.Event) {
	c.queue.Push(ev)
}

func (c *Client) signalChanged() {
	select {
	case c.changed <- struct{}{}:
	default:
	}
}

// pump moves queued events onto the Events channel, ending after Detached.
func (c *Client) pump() {
	defer close(c.events)
	for range c.queue.Ready() {
		for _, ev := range c.queue.Drain() {
			c.events <- ev
			if _, ok := ev.(domain.Detached); ok {
				return
			}
		}
	}
}
