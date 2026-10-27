// Package remote carries the service to clients in other processes. The
// server side bridges each connection to a client of the service running in
// its own process; the client side stands in for the service in the UI's
// process. Both speak the floe protocol.
package remote

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/protocol"
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

const (
	defaultHelloTimeout = 5 * time.Second
	defaultDrainTimeout = 2 * time.Second
)

// Server accepts connections and gives each its own client of the service.
type Server struct {
	connect func() LocalClient
	build   string
	release string
	// helloTimeout is how long a new connection has to introduce itself.
	helloTimeout time.Duration
	// drainTimeout is how long connections get, when the server stops, to
	// pass on their last events before they are cut.
	drainTimeout time.Duration

	mu     sync.Mutex
	conns  map[*protocol.Conn]bool
	closed bool
	wg     sync.WaitGroup
}

// NewServer returns a server that connects each accepted client with
// connect. Only clients from the same build, as named by build, are let in.
// release is the version clients are told the server is.
func NewServer(connect func() LocalClient, build, release string) *Server {
	return &Server{
		connect:      connect,
		build:        build,
		release:      release,
		helloTimeout: defaultHelloTimeout,
		drainTimeout: defaultDrainTimeout,
		conns:        make(map[*protocol.Conn]bool),
	}
}

// Serve accepts connections on l until ctx is done or l fails. It closes l,
// then waits for its connections to end. Connections end by themselves once
// their client of the service is detached, as every client is when the
// service stops; any still open after a short wait are cut.
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
		conn := protocol.NewConn(c)
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

func (s *Server) track(conn *protocol.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.conns[conn] = true
	return true
}

func (s *Server) untrack(conn *protocol.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
}

// shutdown waits for connections to end, cutting any that take too long.
func (s *Server) shutdown() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	ended := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(ended)
	}()
	select {
	case <-ended:
		return
	case <-time.After(s.drainTimeout):
	}
	s.mu.Lock()
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.mu.Unlock()
	<-ended
}

// serve runs one connection: the handshake, then requests until the client
// goes away.
func (s *Server) serve(conn *protocol.Conn, c net.Conn) {
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
		case protocol.Apply:
			client.Apply(m.Command)
		case protocol.GridRequest:
			g, ok := client.Grid(m.Pane, m.Scroll)
			err = conn.Send(protocol.GridReply{ID: m.ID, Grid: g, OK: ok})
		case protocol.StateRequest:
			st, ok := client.State(m.Pane)
			err = conn.Send(protocol.StateReply{ID: m.ID, State: st, OK: ok})
		default:
			err = fmt.Errorf("unexpected %T", m)
		}
		if err != nil {
			return
		}
	}
}

// greet checks the client's Hello and answers it.
func (s *Server) greet(conn *protocol.Conn) error {
	m, err := conn.Receive()
	if err != nil {
		return err
	}
	hello, ok := m.(protocol.Hello)
	var refusal string
	switch {
	case !ok:
		refusal = fmt.Sprintf("expected a hello, got %T", m)
	case hello.Version != protocol.Version:
		refusal = fmt.Sprintf("server speaks protocol %d, client %d", protocol.Version, hello.Version)
	case hello.Build != s.build:
		refusal = ErrOtherBuild.Error()
	}
	if err := conn.Send(protocol.Welcome{Err: refusal, Release: s.release}); err != nil {
		return err
	}
	if refusal != "" {
		return errors.New(refusal)
	}
	return nil
}

// forwardEvents sends the client's events until Detached, the last one,
// then closes the connection: nothing more will come.
func forwardEvents(conn *protocol.Conn, client LocalClient) {
	for ev := range client.Events() {
		if err := conn.Send(protocol.Event{Event: ev}); err != nil {
			_ = conn.Close()
			return
		}
	}
	_ = conn.Close()
}

// forwardChanges passes on change signals until done is closed.
func forwardChanges(conn *protocol.Conn, client LocalClient, done <-chan struct{}) {
	for {
		select {
		case <-client.Changed():
			if err := conn.Send(protocol.Changed{}); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

// ErrOtherBuild is why a server refuses a client from a different build of
// floe. Dial returns it as an *OtherBuildError.
var ErrOtherBuild = errors.New("the floe server is running a different build")

// OtherBuildError is returned by Dial when the server is a different build.
type OtherBuildError struct {
	// Release is the server's version.
	Release string
}

func (e *OtherBuildError) Error() string {
	return fmt.Sprintf("%v (%s)", ErrOtherBuild, e.Release)
}

// Is makes errors.Is(err, ErrOtherBuild) true.
func (e *OtherBuildError) Is(target error) bool { return target == ErrOtherBuild }
