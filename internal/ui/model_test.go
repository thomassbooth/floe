package ui

import (
	"reflect"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/input"
	"github.com/thomassbooth/floe/internal/theme"
)

// fakeBackend records the commands the UI sends and answers state
// questions from a map. Requests are recorded with the applied commands,
// and the test answers them with answer.
type fakeBackend struct {
	applied []domain.Command
	states  map[domain.PaneID]domain.PaneState
	replies []chan domain.Result
}

func (b *fakeBackend) Apply(c domain.Command) { b.applied = append(b.applied, c) }

func (b *fakeBackend) Request(c domain.Command) <-chan domain.Result {
	b.applied = append(b.applied, c)
	reply := make(chan domain.Result, 1)
	b.replies = append(b.replies, reply)
	return reply
}

// answer gives the oldest unanswered request its result.
func (b *fakeBackend) answer(r domain.Result) {
	b.replies[0] <- r
	b.replies = b.replies[1:]
}

func (b *fakeBackend) Events() <-chan domain.Event { return nil }
func (b *fakeBackend) Changed() <-chan struct{}    { return nil }
func (b *fakeBackend) Grid(domain.PaneID, int) (domain.Grid, bool) {
	return domain.Grid{}, false
}

func (b *fakeBackend) State(id domain.PaneID) (domain.PaneState, bool) {
	st, ok := b.states[id]
	return st, ok
}

// sent returns the commands of type T sent so far.
func sent[T domain.Command](b *fakeBackend) []T {
	var out []T
	for _, c := range b.applied {
		if t, ok := c.(T); ok {
			out = append(out, t)
		}
	}
	return out
}

// testWindow is the one window of the screen in these tests: 40x12 at
// (10, 5), on a 100x30 terminal whose status bar is row 29.
var testWindow = domain.Rect{X: 10, Y: 5, W: 40, H: 12}

// client is a UI showing testWindow, with every command it sends recorded.
type client struct {
	t       *testing.T
	m       Model
	backend *fakeBackend
	// waiting holds the work Update handed back, which Bubble Tea would
	// run in the background.
	waiting []tea.Cmd
}

func newClient(t *testing.T) *client {
	b := &fakeBackend{states: map[domain.PaneID]domain.PaneState{1: {}}}
	m := New(b, Options{Keymap: input.DefaultKeymap()})
	m.width, m.height = 100, 30
	m.snap = domain.Snapshot{
		Session: 1,
		Name:    "work",
		Area:    domain.Rect{W: 100, H: 29},
		Focused: 1,
		Windows: []domain.WindowView{{
			ID: 1, Pane: 1, Title: "sh", Focused: true,
			Frame: testWindow, Content: testWindow.Inset(1),
		}},
	}
	return &client{t: t, m: m, backend: b}
}

func (c *client) send(msg tea.Msg) {
	c.t.Helper()
	next, cmd := c.m.Update(msg)
	c.m = next.(Model)
	if cmd != nil {
		c.waiting = append(c.waiting, cmd)
	}
}

// settle runs the work Update handed back, as Bubble Tea would, and feeds
// the messages it gives back into the model.
func (c *client) settle() {
	c.t.Helper()
	for len(c.waiting) > 0 {
		cmd := c.waiting[0]
		c.waiting = c.waiting[1:]
		if msg := cmd(); msg != nil {
			c.send(msg)
		}
	}
}

func (c *client) press(x, y int, mod tea.KeyMod) {
	c.t.Helper()
	c.send(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft, Mod: mod})
}

func (c *client) move(x, y int, mod tea.KeyMod) {
	c.t.Helper()
	c.send(tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft, Mod: mod})
}

func (c *client) release(x, y int, mod tea.KeyMod) {
	c.t.Helper()
	c.send(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft, Mod: mod})
}

func (c *client) key(k tea.Key) {
	c.t.Helper()
	c.send(tea.KeyPressMsg(k))
}

func TestTargetAt(t *testing.T) {
	c := newClient(t)
	// The frame spans columns 10 to 49 and rows 5 to 16. Its buttons sit
	// at columns 42, 44 and 46 of the top row.
	tests := []struct {
		name  string
		x, y  int
		part  part
		edges []domain.Direction
	}{
		{"title", 20, 5, partTitle, nil},
		{"minimize", 42, 5, partMinimize, nil},
		{"maximize", 44, 5, partMaximize, nil},
		{"close", 46, 5, partClose, nil},
		{"space right of close", 47, 5, partClose, nil},
		{"top left corner", 10, 5, partEdge, []domain.Direction{domain.Left, domain.Up}},
		{"top right corner", 49, 5, partEdge, []domain.Direction{domain.Right, domain.Up}},
		{"right edge", 49, 10, partEdge, []domain.Direction{domain.Right}},
		{"bottom edge", 30, 16, partEdge, []domain.Direction{domain.Down}},
		{"bottom left corner", 10, 16, partEdge, []domain.Direction{domain.Left, domain.Down}},
		{"content", 30, 10, partContent, nil},
	}
	for _, tt := range tests {
		got, ok := c.m.targetAt(tt.x, tt.y)
		if !ok || got.part != tt.part || !reflect.DeepEqual(got.edges, tt.edges) {
			t.Errorf("%s: got part %d edges %v, want part %d edges %v", tt.name, got.part, got.edges, tt.part, tt.edges)
		}
	}
	if _, ok := c.m.targetAt(5, 5); ok {
		t.Error("a cell outside every window has no target")
	}
}

func TestTitleBarButtons(t *testing.T) {
	for _, tt := range []struct {
		x    int
		want domain.Command
	}{
		{42, domain.MinimizeWindow{ID: 1}},
		{44, domain.ToggleMaximize{ID: 1}},
		{46, domain.CloseWindow{ID: 1}},
	} {
		c := newClient(t)
		c.press(tt.x, 5, 0)
		if !slices.Contains(c.backend.applied, tt.want) {
			t.Errorf("click at column %d sent %v, want %v", tt.x, c.backend.applied, tt.want)
		}
	}
}

func TestPlainTitleDragDoesNotMove(t *testing.T) {
	c := newClient(t)
	c.press(20, 5, 0)
	c.move(30, 8, 0)
	c.release(30, 8, 0)
	if got := sent[domain.PlaceWindow](c.backend); len(got) != 0 {
		t.Fatalf("a plain drag outside arrange mode moved the window: %v", got)
	}
	if got := sent[domain.FocusWindow](c.backend); len(got) != 1 {
		t.Fatalf("the click should focus the window once, sent %v", c.backend.applied)
	}
}

func TestCtrlTitleDragMovesAndSnaps(t *testing.T) {
	c := newClient(t)
	c.press(20, 5, tea.ModCtrl)
	c.move(20, 5, tea.ModCtrl) // motion within the pressed cell
	if got := sent[domain.PlaceWindow](c.backend); len(got) != 0 {
		t.Fatalf("motion on the pressed cell must not move anything: %v", got)
	}

	c.move(30, 8, tea.ModCtrl)
	want := domain.PlaceWindow{ID: 1, Frame: domain.Rect{X: 20, Y: 8, W: 40, H: 12}}
	if got := sent[domain.PlaceWindow](c.backend); len(got) != 1 || got[0] != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	c.move(0, 14, tea.ModCtrl)
	c.release(0, 14, tea.ModCtrl)
	if got := sent[domain.SnapWindow](c.backend); len(got) != 1 || got[0] != (domain.SnapWindow{ID: 1, Zone: domain.ZoneLeft}) {
		t.Fatalf("dropping on the left edge should snap left, sent %v", got)
	}
	if c.m.drag != nil {
		t.Fatal("the drag should end on release")
	}
}

func TestCtrlEdgeDragResizes(t *testing.T) {
	c := newClient(t)
	c.press(49, 16, tea.ModCtrl) // bottom right corner
	c.move(55, 20, tea.ModCtrl)
	c.release(55, 20, tea.ModCtrl)
	want := []domain.ResizeWindowEdge{
		{ID: 1, Edge: domain.Right, Position: 56},
		{ID: 1, Edge: domain.Down, Position: 21},
	}
	if got := sent[domain.ResizeWindowEdge](c.backend); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := sent[domain.SnapWindow](c.backend); len(got) != 0 {
		t.Fatalf("a resize never snaps: %v", got)
	}
}

func TestProgramThatWantsTheMouseGetsTheDrag(t *testing.T) {
	c := newClient(t)
	c.backend.states[1] = domain.PaneState{Mouse: domain.MouseReportDrags}
	c.press(30, 10, 0)
	c.move(60, 10, 0) // out of the window, still captured
	c.release(60, 10, 0)
	got := sent[domain.SendMouse](c.backend)
	if len(got) != 3 {
		t.Fatalf("want press, move and release forwarded, got %v", got)
	}
	if e := got[1].Event; e.Action != domain.MouseMove || e.X != testWindow.Inset(1).W-1 {
		t.Fatalf("motion outside should be kept at the content's edge, got %+v", e)
	}
	if c.m.selection != nil {
		t.Fatal("a captured drag must not select text")
	}
}

func TestThemePickerPreviewsAndAClickCancels(t *testing.T) {
	c := newClient(t)
	original := c.m.theme
	c.m.runAction(input.ActionThemePicker)
	c.key(tea.Key{Code: tea.KeyDown})
	if c.m.theme.Name == original.Name {
		t.Fatal("moving in the picker should preview the next theme")
	}

	c.press(30, 10, 0)
	if c.m.overlay != nil || c.m.theme.Name != original.Name {
		t.Fatalf("a click should close the picker and restore %s, got overlay %v, theme %s", original.Name, c.m.overlay, c.m.theme.Name)
	}
	if len(c.backend.applied) != 0 {
		t.Fatalf("the click that closes the picker goes no further, sent %v", c.backend.applied)
	}
}

func TestThemePickerKeepsWithEnter(t *testing.T) {
	c := newClient(t)
	c.m.runAction(input.ActionThemePicker)
	c.key(tea.Key{Code: tea.KeyDown})
	previewed := c.m.theme
	c.key(tea.Key{Code: tea.KeyEnter})
	if c.m.overlay != nil || c.m.theme.Name != previewed.Name || !c.m.themeChosen {
		t.Fatalf("enter should keep %s, got %s", previewed.Name, c.m.theme.Name)
	}
	if previewed.Name == theme.Default {
		t.Fatal("the test should move off the default theme")
	}
}

func TestSwitcherClosesOnAClickThatGoesNoFurther(t *testing.T) {
	c := newClient(t)
	c.m.runAction(input.ActionSessionSwitcher)
	c.press(20, 5, 0)
	if c.m.overlay != nil {
		t.Fatal("a click should close the switcher")
	}
	if len(c.backend.applied) != 0 {
		t.Fatalf("the click that closes the switcher goes no further, sent %v", c.backend.applied)
	}
}

func TestPromptCoversTheStatusBarButNotTheWindows(t *testing.T) {
	c := newClient(t)
	c.m.runAction(input.ActionRenameSession)

	c.press(0, 29, 0) // where the session badge would be
	if _, ok := c.m.overlay.(*prompt); !ok {
		t.Fatalf("a click on the prompt should leave it open, overlay is %v", c.m.overlay)
	}
	if len(c.backend.applied) != 0 {
		t.Fatalf("the status bar under the prompt should not be clicked, sent %v", c.backend.applied)
	}

	c.press(30, 10, 0)
	if got := sent[domain.FocusWindow](c.backend); len(got) != 1 {
		t.Fatalf("windows can be clicked while the prompt is open, sent %v", c.backend.applied)
	}

	c.key(tea.Key{Code: 'u', Mod: tea.ModCtrl})
	c.key(tea.Key{Code: 'x', Text: "x"})
	c.key(tea.Key{Code: tea.KeyEnter})
	if got := sent[domain.RenameSession](c.backend); len(got) != 1 || got[0].Name != "x" {
		t.Fatalf("enter should rename to x, sent %v", c.backend.applied)
	}
	c.backend.answer(domain.Result{})
	c.settle()
	if c.m.overlay != nil {
		t.Fatal("the prompt should close once the rename goes through")
	}
}

func TestPromptDrawsOverTheStatusBarWithItsCursor(t *testing.T) {
	c := newClient(t)
	c.m.runAction(input.ActionRenameSession)
	_, cursor := c.m.render()
	if cursor == nil || cursor.Y != 29 {
		t.Fatalf("the cursor should be in the prompt on row 29, got %+v", cursor)
	}
}

func TestPromptStaysOpenWithTheReasonWhenRefused(t *testing.T) {
	c := newClient(t)
	c.m.runAction(input.ActionRenameSession)
	c.key(tea.Key{Code: 'u', Mod: tea.ModCtrl})
	c.key(tea.Key{Code: '1', Text: "1"})
	c.key(tea.Key{Code: tea.KeyEnter})
	if got := sent[domain.RenameSession](c.backend); len(got) != 1 || got[0].Name != "1" {
		t.Fatalf("enter should ask to rename to 1, sent %v", c.backend.applied)
	}

	c.backend.answer(domain.Result{Err: domain.ErrSessionExists})
	c.settle()
	p, ok := c.m.overlay.(*prompt)
	if !ok || p.err != domain.ErrSessionExists.Error() || string(p.text) != "1" {
		t.Fatalf("the prompt should stay open with the name and the reason, got %#v", c.m.overlay)
	}
	if c.m.notice != "" {
		t.Fatalf("the reason belongs in the prompt, not the status bar: %q", c.m.notice)
	}

	c.key(tea.Key{Code: 'x', Text: "x"})
	if p.err != "" {
		t.Fatal("typing should clear the reason")
	}
	c.key(tea.Key{Code: tea.KeyEnter})
	c.backend.answer(domain.Result{})
	c.settle()
	if c.m.overlay != nil {
		t.Fatal("the prompt should close once the rename goes through")
	}
}

func TestResultAfterThePromptClosedShowsInTheStatusBar(t *testing.T) {
	c := newClient(t)
	c.m.runAction(input.ActionRenameSession)
	c.key(tea.Key{Code: tea.KeyEnter})
	c.key(tea.Key{Code: tea.KeyEscape})
	c.backend.answer(domain.Result{Err: domain.ErrSessionExists})
	c.settle()
	if c.m.overlay != nil || c.m.notice != domain.ErrSessionExists.Error() {
		t.Fatalf("a late failure should show as a notice, got overlay %v, notice %q", c.m.overlay, c.m.notice)
	}
}
