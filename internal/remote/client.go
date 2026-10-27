package remote

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/queue"
	"github.com/thomasbooth/multiplexer/internal/wire"
)

// ErrLostServer is why a client is detached when its connection to the
// server breaks.
var ErrLostServer = errors.New("lost the connection to the mux server")

// Client stands in for the service in the UI's process: it offers what the
// UI needs, over a connection to the server. Its methods are safe to call
// from any goroutine.
type Client struct {
	conn *wire.Conn

	// out holds messages waiting to be written, so Apply never blocks.
	out *queue.Unbounded[wire.Message]
	// in holds events waiting for the UI to take them.
	in      *queue.Unbounded[domain.Event]
	events  chan domain.Event
	changed chan struct{}

	mu      sync.Mutex
	nextID  uint64
	waiting map[uint64]chan wire.Message

	// closing is set by Close, so the connection ending is not a failure.
	closing atomic.Bool
	quit    chan struct{} // closed by Close: write what is queued and stop
	done    chan struct{} // closed when the connection is gone
	once    sync.Once
}

// Dial introduces itself to the server over rwc as build and returns the
// connected client. It fails with ErrOtherBuild when the server runs a
// different build.
func Dial(rwc io.ReadWriteCloser, build string) (*Client, error) {
	conn := wire.NewConn(rwc)
	if err := handshake(conn, build); err != nil {
		_ = conn.Close()
		return nil, err
	}
	c := &Client{
		conn:    conn,
		out:     queue.New[wire.Message](),
		in:      queue.New[domain.Event](),
		events:  make(chan domain.Event),
		changed: make(chan struct{}, 1),
		waiting: make(map[uint64]chan wire.Message),
		quit:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go c.read()
	go c.write()
	go c.pump()
	return c, nil
}

func handshake(conn *wire.Conn, build string) error {
	if err := conn.Send(wire.Hello{Protocol: wire.Protocol, Build: build}); err != nil {
		return fmt.Errorf("greet the mux server: %w", err)
	}
	m, err := conn.Receive()
	if err != nil {
		return fmt.Errorf("greet the mux server: %w", err)
	}
	welcome, ok := m.(wire.Welcome)
	switch {
	case !ok:
		return fmt.Errorf("greet the mux server: unexpected %T", m)
	case welcome.Err == ErrOtherBuild.Error():
		return ErrOtherBuild
	case welcome.Err != "":
		return fmt.Errorf("the mux server refused: %s", welcome.Err)
	}
	return nil
}

// Apply queues a command for the server. It never blocks.
func (c *Client) Apply(cmd domain.Command) {
	c.out.Push(wire.Apply{Command: cmd})
}

// Events receives everything the server tells this client. It is closed
// after Detached, which also comes if the connection breaks.
func (c *Client) Events() <-chan domain.Event { return c.events }

// Changed receives a signal when a pane in the client's session changes.
// Signals coalesce.
func (c *Client) Changed() <-chan struct{} { return c.changed }

// Grid asks the server for a pane's screen, scrolled scroll lines up into
// its history.
func (c *Client) Grid(id domain.PaneID, scroll int) (domain.Grid, bool) {
	m, ok := c.ask(func(req uint64) wire.Message {
		return wire.GridRequest{ID: req, Pane: id, Scroll: scroll}
	})
	if !ok {
		return domain.Grid{}, false
	}
	r := m.(wire.GridReply)
	return r.Grid, r.OK
}

// State asks the server about a pane's history and input modes.
func (c *Client) State(id domain.PaneID) (domain.PaneState, bool) {
	m, ok := c.ask(func(req uint64) wire.Message {
		return wire.StateRequest{ID: req, Pane: id}
	})
	if !ok {
		return domain.PaneState{}, false
	}
	r := m.(wire.StateReply)
	return r.State, r.OK
}

// Close sends what is queued, then disconnects. The client's sessions keep
// running on the server.
func (c *Client) Close() {
	c.once.Do(func() {
		c.closing.Store(true)
		close(c.quit)
	})
	<-c.done
}

// ask sends the request made by build and waits for its reply. It returns
// false if the connection ends first.
func (c *Client) ask(build func(req uint64) wire.Message) (wire.Message, bool) {
	reply := make(chan wire.Message, 1)
	c.mu.Lock()
	c.nextID++
	req := c.nextID
	c.waiting[req] = reply
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.waiting, req)
		c.mu.Unlock()
	}()

	c.out.Push(build(req))
	select {
	case m := <-reply:
		return m, true
	case <-c.done:
		return nil, false
	}
}

// answer hands a reply to whoever asked for it.
func (c *Client) answer(req uint64, m wire.Message) {
	c.mu.Lock()
	reply, ok := c.waiting[req]
	c.mu.Unlock()
	if ok {
		reply <- m
	}
}

// read handles messages from the server until the connection ends, then
// makes sure the UI hears it is detached.
func (c *Client) read() {
	defer close(c.done)
	detached := false
	for !detached {
		m, err := c.conn.Receive()
		if err != nil {
			break
		}
		switch m := m.(type) {
		case wire.Event:
			c.in.Push(m.Event)
			_, detached = m.Event.(domain.Detached)
		case wire.Changed:
			select {
			case c.changed <- struct{}{}:
			default:
			}
		case wire.GridReply:
			c.answer(m.ID, m)
		case wire.StateReply:
			c.answer(m.ID, m)
		}
	}
	_ = c.conn.Close()
	if !detached {
		var cause error
		if !c.closing.Load() {
			cause = ErrLostServer
		}
		c.in.Push(domain.Detached{Err: cause})
	}
}

// write sends queued messages until the connection ends, or Close asks it
// to finish.
func (c *Client) write() {
	for {
		select {
		case <-c.out.Ready():
			if !c.flush() {
				return
			}
		case <-c.quit:
			c.flush()
			_ = c.conn.Close()
			return
		case <-c.done:
			return
		}
	}
}

// flush writes everything queued, reporting whether the connection still
// works.
func (c *Client) flush() bool {
	for _, m := range c.out.Drain() {
		if err := c.conn.Send(m); err != nil {
			_ = c.conn.Close()
			return false
		}
	}
	return true
}

// pump moves queued events onto the Events channel, ending after Detached.
func (c *Client) pump() {
	defer close(c.events)
	for range c.in.Ready() {
		for _, ev := range c.in.Drain() {
			c.events <- ev
			if _, ok := ev.(domain.Detached); ok {
				return
			}
		}
	}
}
