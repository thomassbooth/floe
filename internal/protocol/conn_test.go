package protocol

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/thomassbooth/floe/internal/domain"
)

func pipe(t testing.TB) (*Conn, *Conn) {
	t.Helper()
	a, b := net.Pipe()
	ca, cb := NewConn(a), NewConn(b)
	t.Cleanup(func() {
		_ = ca.Close()
		_ = cb.Close()
	})
	return ca, cb
}

// roundTrip sends m from one end and returns what the other end received.
func roundTrip(t *testing.T, m Message) Message {
	t.Helper()
	a, b := pipe(t)
	errc := make(chan error, 1)
	go func() { errc <- a.Send(m) }()
	got, err := b.Receive()
	if err != nil {
		t.Fatalf("receive %T: %v", m, err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("send %T: %v", m, err)
	}
	return got
}

func TestEveryCommandCrosses(t *testing.T) {
	r := domain.Rect{X: 1, Y: 2, W: 30, H: 10}
	samples := []domain.Command{
		domain.SetArea{Size: domain.Size{W: 80, H: 23}},
		domain.AttachSession{Name: "work"},
		domain.CreateSession{Name: "play"},
		domain.SwitchSession{ID: 3},
		domain.RenameSession{Name: "renamed"},
		domain.KillSession{ID: 2},
		domain.NewWindow{Frame: r},
		domain.SplitWindow{Dir: domain.Right},
		domain.FocusDirection{Dir: domain.Up},
		domain.FocusWindow{ID: 4},
		domain.ResizeWindow{Dir: domain.Left, Amount: 2},
		domain.MoveWindow{Dir: domain.Down, Amount: 1},
		domain.CycleFocus{Delta: -1},
		domain.PlaceWindow{ID: 2, Frame: r},
		domain.ResizeWindowEdge{ID: 2, Edge: domain.Right, Position: 40},
		domain.ToggleMaximize{ID: 1},
		domain.SnapWindow{ID: 1, Zone: domain.ZoneRight},
		domain.Tidy{},
		domain.CloseWindow{ID: 5},
		domain.SendKey{Key: domain.Key{Code: 'é', Text: "é", Mod: domain.ModAlt}, Window: 2},
		domain.SendMouse{Window: 1, Event: domain.MouseEvent{Action: domain.MouseWheel, Button: domain.WheelUp, X: 3, Y: 4, Mod: domain.ModShift}},
		domain.Paste{Text: "pasted\ntext"},
		domain.MinimizeWindow{ID: 3},
		domain.RestoreMinimized{},
	}
	if len(samples) != len(commands) {
		t.Fatalf("%d samples for %d registered commands", len(samples), len(commands))
	}
	for _, cmd := range samples {
		got := roundTrip(t, Apply{Command: cmd})
		if !reflect.DeepEqual(got, Apply{Command: cmd}) {
			t.Errorf("sent %#v, got %#v", cmd, got)
		}
	}
}

func TestEveryEventCrosses(t *testing.T) {
	snap := domain.Snapshot{
		Session: 1,
		Name:    "main",
		Area:    domain.Rect{W: 80, H: 23},
		Windows: []domain.WindowView{{
			ID: 1, Pane: 7, Title: "zsh", Busy: true, Bell: true,
			Frame:   domain.Rect{W: 40, H: 23},
			Content: domain.Rect{X: 1, Y: 1, W: 38, H: 21},
			Focused: true, Maximized: true, Minimized: true,
			Restore: domain.Rect{X: 2, Y: 2, W: 20, H: 10},
		}},
		Focused: 1,
	}
	samples := []domain.Event{
		domain.LayoutChanged{Snapshot: snap},
		domain.SessionsChanged{Sessions: []domain.SessionInfo{{ID: 1, Name: "main", Windows: 2, Clients: 1}}, Current: 1},
		domain.ClipboardSet{Text: "copied"},
		domain.CommandFailed{Command: domain.FocusWindow{ID: 9}},
		domain.Detached{},
	}
	if len(samples) != len(events) {
		t.Fatalf("%d samples for %d registered events", len(samples), len(events))
	}
	for _, ev := range samples {
		got := roundTrip(t, Event{Event: ev})
		if !reflect.DeepEqual(got, Event{Event: ev}) {
			t.Errorf("sent %#v, got %#v", ev, got)
		}
	}
}

func TestErrorsCrossAsText(t *testing.T) {
	got := roundTrip(t, Event{Event: domain.CommandFailed{
		Command: domain.FocusWindow{ID: 9},
		Err:     domain.ErrUnknownWindow,
	}})
	failed := got.(Event).Event.(domain.CommandFailed)
	if failed.Err == nil || failed.Err.Error() != domain.ErrUnknownWindow.Error() {
		t.Fatalf("error = %v, want %v", failed.Err, domain.ErrUnknownWindow)
	}

	got = roundTrip(t, Event{Event: domain.Detached{Err: errors.New("shell would not start")}})
	if err := got.(Event).Event.(domain.Detached).Err; err == nil || err.Error() != "shell would not start" {
		t.Fatalf("detached error = %v", err)
	}
}

func TestCommandRequestsAndRepliesCross(t *testing.T) {
	req := CommandRequest{ID: 7, Command: domain.RenameSession{Name: "work"}}
	if got := roundTrip(t, req); !reflect.DeepEqual(got, req) {
		t.Fatalf("got %#v, want %#v", got, req)
	}

	got := roundTrip(t, CommandReply{ID: 7, Result: domain.Result{Err: domain.ErrSessionExists}})
	reply := got.(CommandReply)
	if reply.ID != 7 || reply.Result.Err == nil || reply.Result.Err.Error() != domain.ErrSessionExists.Error() {
		t.Fatalf("got %#v, want the error's text under ID 7", reply)
	}

	if got := roundTrip(t, CommandReply{ID: 8}).(CommandReply); got.Result.Err != nil {
		t.Fatalf("a success should cross as no error, got %v", got.Result.Err)
	}
}

func TestGridsAndStatesCross(t *testing.T) {
	grid := domain.Grid{
		Size: domain.Size{W: 3, H: 1},
		Cells: []domain.Cell{
			{Content: "a", Width: 1, Style: domain.Style{Fg: domain.BasicColor(1), Attrs: domain.AttrBold}},
			{Content: "界", Width: 2, Style: domain.Style{Bg: domain.RGBColor(1, 2, 3), Underline: domain.UnderlineCurly, UnderlineColor: domain.IndexedColor(99)}},
			{Width: 0},
		},
		Cursor:        domain.Point{X: 2},
		CursorVisible: true,
	}
	for _, m := range []Message{
		Hello{Version: Version, Build: "abc"},
		Welcome{Err: "different build", Release: "v1.2.3"},
		Changed{},
		GridRequest{ID: 1, Pane: 2, Scroll: 3},
		GridReply{ID: 1, Grid: grid, OK: true},
		StateRequest{ID: 2, Pane: 2},
		StateReply{ID: 2, State: domain.PaneState{Scrollback: 10, AltScreen: true, Mouse: domain.MouseReportDrags}, OK: true},
	} {
		if got := roundTrip(t, m); !reflect.DeepEqual(got, m) {
			t.Errorf("sent %#v, got %#v", m, got)
		}
	}
}

func TestConcurrentSendsArriveWhole(t *testing.T) {
	a, b := pipe(t)
	const senders, each = 4, 50
	var wg sync.WaitGroup
	for s := range senders {
		wg.Go(func() {
			for i := range each {
				if err := a.Send(GridRequest{ID: uint64(s*each + i)}); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	seen := make(map[uint64]bool)
	for range senders * each {
		m, err := b.Receive()
		if err != nil {
			t.Fatal(err)
		}
		seen[m.(GridRequest).ID] = true
	}
	wg.Wait()
	if len(seen) != senders*each {
		t.Fatalf("got %d distinct messages, want %d", len(seen), senders*each)
	}
}

func TestReceiveReportsEOFWhenClosed(t *testing.T) {
	a, b := pipe(t)
	_ = a.Close()
	if _, err := b.Receive(); !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want EOF", err)
	}
}

// TestEveryDomainTypeIsRegistered keeps the registered lists in step with
// the domain package: a command or event type nobody registered would fail
// to send at run time.
func TestEveryDomainTypeIsRegistered(t *testing.T) {
	pkgs, err := parser.ParseDir(token.NewFileSet(), "../domain", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	implementers := map[string][]string{}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Recv == nil {
					continue
				}
				if ident, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
					implementers[fn.Name.Name] = append(implementers[fn.Name.Name], ident.Name)
				}
			}
		}
	}
	check := func(method string, registered []string) {
		want := implementers[method]
		slices.Sort(want)
		slices.Sort(registered)
		if !slices.Equal(want, registered) {
			t.Errorf("domain types with %s(): %v\nregistered: %v", method, want, registered)
		}
	}
	check("command", typeNames(commands))
	check("event", typeNames(events))
}

func typeNames[T any](values []T) []string {
	var names []string
	for _, v := range values {
		names = append(names, reflect.TypeOf(v).Name())
	}
	return names
}

func TestGridRepliesSurviveEdgeCases(t *testing.T) {
	for _, g := range []domain.Grid{
		{},
		{Size: domain.Size{W: 2, H: 2}, Cells: []domain.Cell{
			{Content: " ", Width: 1},
			{Content: "", Width: 1},
			{Content: "👍", Width: 2, Style: domain.Style{Fg: domain.RGBColor(255, 255, 255)}},
			{Content: "", Width: 0, Style: domain.Style{Fg: domain.RGBColor(255, 255, 255)}},
		}, Cursor: domain.Point{X: -1, Y: 5}},
	} {
		m := GridReply{ID: 1 << 40, Grid: g, OK: true}
		if got := roundTrip(t, m); !reflect.DeepEqual(got, m) {
			t.Errorf("sent %#v, got %#v", m, got)
		}
	}
}

func TestGridRepliesRejectCorruptData(t *testing.T) {
	good, err := GridReply{Grid: domain.Grid{Size: domain.Size{W: 1, H: 1}, Cells: []domain.Cell{{Content: "x", Width: 1}}}}.GobEncode()
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{
		good[:len(good)-1],
		append(good, 0),
		{0, 0, 0, 0xfe, 0xff, 0xff, 0xff, 0x0f, 2, 0, 0}, // huge width
	} {
		var r GridReply
		if err := r.GobDecode(data); err == nil {
			t.Errorf("decoded %v without error", data)
		}
	}
	if _, err := (GridReply{Grid: domain.Grid{Size: domain.Size{W: 2, H: 2}}}).GobEncode(); err == nil {
		t.Error("encoded a grid whose cells do not match its size")
	}
}
