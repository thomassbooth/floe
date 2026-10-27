// Package remote carries the service to clients in other processes. The
// server side bridges each connection to a client of the service running in
// its own process; the client side stands in for the service in the UI's
// process. Both speak the wire protocol.
package remote

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/wire"
)

// LocalClient is a client of the service in the server's process.
type LocalClient interface {
	Apply(domain.Command)
	Events() <-chan domain.Event
	Changed() <-chan struct{}
	Grid(id domain.PaneID, scroll int) (domain.Grid, bool)
	State(id domain.PaneID) (domain.PaneState, bool)
	Close()
}

const defaultHelloTimeout = 5 * time.Second

// Server accepts connections and gives each its own client of the service.
type Server struct {
	connect func() LocalClient
	build   string
	// helloTimeout is how long a new connection has to introduce itself.
	helloTimeout time.Duration

	mu     sync.Mutex
	conns  map[*wire.Conn]bool
	closed bool
	wg     sync.WaitGroup
}

// NewServer returns a server that connects each accepted client with
// connect. Only clients from the same build, as named by build, are let in.
func NewServer(connect func() LocalClient, build string) *Server {
	return &Server{
		connect:      connect,
		build:        build,
		helloTimeout: defaultHelloTimeout,
		conns:        make(map[*wire.Conn]bool),
	}
}

// Serve accepts connections on l until ctx is done or l fails. It closes l,
// and every connection, before returning.
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	defer stop()
	defer s.shutdown()
	for {
		c, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		conn := wire.NewConn(c)
		if !s.track(conn) {
			_ = conn.Close()
			return nil
		}
		s.wg.Go(func() {
			defer s.untrack(conn)
			s.serve(conn, c)
		})
	}
}

func (s *Server) track(conn *wire.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.conns[conn] = true
	return true
}

func (s *Server) untrack(conn *wire.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
}

// shutdown closes every connection and waits for their clients to go.
func (s *Server) shutdown() {
	s.mu.Lock()
	s.closed = true
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

// serve runs one connection: the handshake, then requests until the client
// goes away.
func (s *Server) serve(conn *wire.Conn, c net.Conn) {
	defer conn.Close()
	_ = c.SetReadDeadline(time.Now().Add(s.helloTimeout))
	if err := s.greet(conn); err != nil {
		return
	}
	_ = c.SetReadDeadline(time.Time{})

	client := s.connect()
	done := make(chan struct{})
	var wg sync.WaitGroup
	defer func() {
		close(done)
		client.Close()
		_ = conn.Close()
		wg.Wait()
	}()
	wg.Go(func() { forwardEvents(conn, client) })
	wg.Go(func() { forwardChanges(conn, client, done) })

	for {
		m, err := conn.Receive()
		if err != nil {
			return
		}
		switch m := m.(type) {
		case wire.Apply:
			client.Apply(m.Command)
		case wire.GridRequest:
			g, ok := client.Grid(m.Pane, m.Scroll)
			err = conn.Send(wire.GridReply{ID: m.ID, Grid: g, OK: ok})
		case wire.StateRequest:
			st, ok := client.State(m.Pane)
			err = conn.Send(wire.StateReply{ID: m.ID, State: st, OK: ok})
		default:
			err = fmt.Errorf("unexpected %T", m)
		}
		if err != nil {
			return
		}
	}
}

// greet checks the client's Hello and answers it.
func (s *Server) greet(conn *wire.Conn) error {
	m, err := conn.Receive()
	if err != nil {
		return err
	}
	hello, ok := m.(wire.Hello)
	var refusal string
	switch {
	case !ok:
		refusal = fmt.Sprintf("expected a hello, got %T", m)
	case hello.Protocol != wire.Protocol:
		refusal = fmt.Sprintf("server speaks protocol %d, client %d", wire.Protocol, hello.Protocol)
	case hello.Build != s.build:
		refusal = ErrOtherBuild.Error()
	}
	if err := conn.Send(wire.Welcome{Err: refusal}); err != nil {
		return err
	}
	if refusal != "" {
		return errors.New(refusal)
	}
	return nil
}

// forwardEvents sends the client's events until Detached, the last one,
// then closes the connection: nothing more will come.
func forwardEvents(conn *wire.Conn, client LocalClient) {
	for ev := range client.Events() {
		if err := conn.Send(wire.Event{Event: ev}); err != nil {
			_ = conn.Close()
			return
		}
	}
	_ = conn.Close()
}

// forwardChanges passes on change signals until done is closed.
func forwardChanges(conn *wire.Conn, client LocalClient, done <-chan struct{}) {
	for {
		select {
		case <-client.Changed():
			if err := conn.Send(wire.Changed{}); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

// ErrOtherBuild is why a server refuses a client from a different build of
// mux.
var ErrOtherBuild = errors.New("the mux server is running a different build")
