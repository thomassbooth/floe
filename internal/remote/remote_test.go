package remote

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/thomassbooth/floe/internal/domain"
)

// fakeLocal is a client of a pretend service.
type fakeLocal struct {
	applied chan domain.Command
	events  chan domain.Event
	changed chan struct{}
	closed  chan struct{}

	detachOnce, closeOnce sync.Once
}

func newFakeLocal() *fakeLocal {
	return &fakeLocal{
		applied: make(chan domain.Command, 64),
		events:  make(chan domain.Event, 64),
		changed: make(chan struct{}, 1),
		closed:  make(chan struct{}),
	}
}

func (f *fakeLocal) Apply(cmd domain.Command)    { f.applied <- cmd }
func (f *fakeLocal) Events() <-chan domain.Event { return f.events }
func (f *fakeLocal) Changed() <-chan struct{}    { return f.changed }
func (f *fakeLocal) emit(ev domain.Event)        { f.events <- ev }
func (f *fakeLocal) signal()                     { f.changed <- struct{}{} }

// detach ends the client's events, as the service does.
func (f *fakeLocal) detach(err error) {
	f.detachOnce.Do(func() {
		f.events <- domain.Detached{Err: err}
		close(f.events)
	})
}

func (f *fakeLocal) Close() {
	f.detach(nil)
	f.closeOnce.Do(func() { close(f.closed) })
}

func (f *fakeLocal) State(id domain.PaneID) (domain.PaneState, bool) {
	if id != 1 {
		return domain.PaneState{}, false
	}
	return domain.PaneState{Scrollback: 42, Mouse: domain.MouseReportClicks}, true
}

func (f *fakeLocal) Grid(id domain.PaneID, scroll int) (domain.Grid, bool) {
	if id != 1 {
		return domain.Grid{}, false
	}
	return domain.Grid{
		Size:  domain.Size{W: 1, H: 1},
		Cells: []domain.Cell{{Content: string(rune('0' + scroll)), Width: 1}},
	}, true
}

type rig struct {
	t      *testing.T
	path   string
	server *Server
	cancel context.CancelFunc
	served chan error
	locals chan *fakeLocal
}

// newRig starts a server for build on a fresh socket.
func newRig(t *testing.T, build string, opts ...func(*Server)) *rig {
	t.Helper()
	// Socket paths are short on macOS, so avoid the long t.TempDir.
	dir, err := os.MkdirTemp("", "floe")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	r := &rig{t: t, path: filepath.Join(dir, "s"), served: make(chan error, 1), locals: make(chan *fakeLocal, 8)}
	l, err := net.Listen("unix", r.path)
	if err != nil {
		t.Fatal(err)
	}
	r.server = NewServer(func() LocalClient {
		f := newFakeLocal()
		r.locals <- f
		return f
	}, build, "v9.9.9")
	r.server.drainTimeout = 100 * time.Millisecond
	for _, opt := range opts {
		opt(r.server)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go func() { r.served <- r.server.Serve(ctx, l) }()
	t.Cleanup(r.stop)
	return r
}

func (r *rig) stop() {
	r.cancel()
	select {
	case err := <-r.served:
		if err != nil {
			r.t.Errorf("serve: %v", err)
		}
		r.served <- nil
	case <-time.After(5 * time.Second):
		r.t.Fatal("server did not stop")
	}
}

func (r *rig) dialConn() net.Conn {
	r.t.Helper()
	c, err := net.Dial("unix", r.path)
	if err != nil {
		r.t.Fatal(err)
	}
	return c
}

// attach connects a client and returns it with its server-side partner.
func (r *rig) attach() (*Client, *fakeLocal) {
	r.t.Helper()
	c, err := Dial(r.dialConn(), "build-1")
	if err != nil {
		r.t.Fatal(err)
	}
	r.t.Cleanup(c.Close)
	select {
	case f := <-r.locals:
		return c, f
	case <-time.After(5 * time.Second):
		r.t.Fatal("server never connected the client")
		return nil, nil
	}
}

func next[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v, ok := <-ch:
		if !ok {
			t.Fatal("channel closed")
		}
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
	var zero T
	return zero
}

func closedSoon[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected the channel to be closed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("channel never closed")
	}
}

func TestClientTalksToTheServiceThroughTheServer(t *testing.T) {
	r := newRig(t, "build-1")
	c, f := r.attach()

	c.Apply(domain.AttachSession{Name: "work"})
	c.Apply(domain.SendKey{Key: domain.Key{Code: 'x', Text: "x"}})
	if got := next(t, f.applied); got != (domain.AttachSession{Name: "work"}) {
		t.Fatalf("first command = %#v", got)
	}
	if got := next(t, f.applied); got != (domain.SendKey{Key: domain.Key{Code: 'x', Text: "x"}}) {
		t.Fatalf("second command = %#v", got)
	}

	f.emit(domain.ClipboardSet{Text: "hi"})
	if got := next(t, c.Events()); got != (domain.ClipboardSet{Text: "hi"}) {
		t.Fatalf("event = %#v", got)
	}

	f.signal()
	next(t, c.Changed())

	g, ok := c.Grid(1, 3)
	if !ok || g.At(0, 0).Content != "3" {
		t.Fatalf("grid = %#v, %v", g, ok)
	}
	if _, ok := c.Grid(2, 0); ok {
		t.Fatal("grid of a missing pane should not be ok")
	}
	st, ok := c.State(1)
	if !ok || st.Scrollback != 42 || st.Mouse != domain.MouseReportClicks {
		t.Fatalf("state = %#v, %v", st, ok)
	}
}

func TestClientsAreServedSideBySide(t *testing.T) {
	r := newRig(t, "build-1")
	a, fa := r.attach()
	b, fb := r.attach()
	a.Apply(domain.Tidy{})
	b.Apply(domain.CycleFocus{Delta: 1})
	if got := next(t, fa.applied); got != (domain.Tidy{}) {
		t.Fatalf("a's command = %#v", got)
	}
	if got := next(t, fb.applied); got != (domain.CycleFocus{Delta: 1}) {
		t.Fatalf("b's command = %#v", got)
	}
}

func TestCloseSendsWhatIsQueuedThenDisconnects(t *testing.T) {
	r := newRig(t, "build-1")
	c, f := r.attach()
	for i := range 20 {
		c.Apply(domain.CycleFocus{Delta: i})
	}
	c.Close()
	for i := range 20 {
		if got := next(t, f.applied); got != (domain.CycleFocus{Delta: i}) {
			t.Fatalf("command %d = %#v", i, got)
		}
	}
	closedSoon(t, f.closed)

	ev := next(t, c.Events())
	if d, ok := ev.(domain.Detached); !ok || d.Err != nil {
		t.Fatalf("last event = %#v, want a clean Detached", ev)
	}
	closedSoon(t, c.Events())
	if _, ok := c.Grid(1, 0); ok {
		t.Fatal("a closed client should not get grids")
	}
}

func TestDetachedFromTheServiceReachesTheClient(t *testing.T) {
	r := newRig(t, "build-1")
	c, f := r.attach()
	f.detach(errors.New("shell would not start"))
	ev := next(t, c.Events())
	if d, ok := ev.(domain.Detached); !ok || d.Err == nil || d.Err.Error() != "shell would not start" {
		t.Fatalf("event = %#v", ev)
	}
	closedSoon(t, c.Events())
	closedSoon(t, f.closed)
}

func TestLosingTheServerDetachesWithAnError(t *testing.T) {
	r := newRig(t, "build-1")
	c, f := r.attach()
	r.stop()
	closedSoon(t, f.closed)
	var last domain.Event
	for ev := range c.Events() {
		last = ev
	}
	if d, ok := last.(domain.Detached); !ok || !errors.Is(d.Err, ErrLostServer) {
		t.Fatalf("last event = %#v, want Detached with ErrLostServer", last)
	}
}

func TestServerRefusesOtherBuilds(t *testing.T) {
	r := newRig(t, "build-1")
	_, err := Dial(r.dialConn(), "build-2")
	var other *OtherBuildError
	if !errors.Is(err, ErrOtherBuild) || !errors.As(err, &other) || other.Release != "v9.9.9" {
		t.Fatalf("err = %v, want an OtherBuildError naming v9.9.9", err)
	}
	select {
	case <-r.locals:
		t.Fatal("a refused client must not reach the service")
	default:
	}
}

func TestServerDropsConnectionsThatNeverSayHello(t *testing.T) {
	r := newRig(t, "build-1", func(s *Server) { s.helloTimeout = 50 * time.Millisecond })
	conn := r.dialConn()
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("read = %v, want the server to hang up", err)
	}
}

func TestStoppingLetsClientsHearTheyAreDetached(t *testing.T) {
	r := newRig(t, "build-1", func(s *Server) { s.drainTimeout = 5 * time.Second })
	c, f := r.attach()
	r.cancel()
	// The service detaches every client as it stops, a moment after the
	// server was told to stop.
	time.Sleep(50 * time.Millisecond)
	f.detach(nil)
	ev := next(t, c.Events())
	if d, ok := ev.(domain.Detached); !ok || d.Err != nil {
		t.Fatalf("event = %#v, want a clean Detached", ev)
	}
}
