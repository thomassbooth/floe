package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

func gridText(g domain.Grid) string {
	var b strings.Builder
	for y := range g.Size.H {
		for x := range g.Size.W {
			c := g.At(x, y)
			if c.Width == 0 {
				continue
			}
			b.WriteString(c.Content)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func waitForText(t *testing.T, h *Host, id domain.PaneID, want string) domain.Grid {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		g, ok := h.Grid(id, 0)
		if ok && strings.Contains(gridText(g), want) {
			return g
		}
		if time.Now().After(deadline) {
			t.Fatalf("screen never showed %q:\n%s", want, gridText(g))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitExited(t *testing.T, h *Host, id domain.PaneID) {
	t.Helper()
	select {
	case got := <-h.Exited():
		if got != id {
			t.Fatalf("pane %d exited, want %d", got, id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pane did not exit")
	}
	if _, ok := h.Grid(id, 0); ok {
		t.Fatal("exited pane should be forgotten")
	}
}

func newTestHost(t *testing.T) *Host {
	h := NewHost(Options{Shell: "/bin/sh", Env: []string{"PS1=$ ", "ENV="}})
	t.Cleanup(h.Close)
	return h
}

func TestHostRunsShellAndTypesKeys(t *testing.T) {
	h := newTestHost(t)
	if err := h.Spawn(1, domain.Size{W: 40, H: 10}, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Kill(1) })

	for _, r := range "echo $((6*7))" {
		h.SendKey(1, domain.Key{Code: r, Text: string(r)})
	}
	h.SendKey(1, domain.Key{Code: domain.KeyEnter})
	waitForText(t, h, 1, "42")

	select {
	case <-h.Changed():
	default:
		t.Fatal("expected a change signal after output")
	}
	if got := h.TakeChanged(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("changed panes = %v, want [1]", got)
	}
}

func TestHostResizeReachesTheShell(t *testing.T) {
	h := newTestHost(t)
	if err := h.Spawn(1, domain.Size{W: 40, H: 10}, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Kill(1) })

	h.Resize(1, domain.Size{W: 50, H: 12})
	h.Paste(1, "stty size\r")
	g := waitForText(t, h, 1, "12 50")
	if g.Size != (domain.Size{W: 50, H: 12}) {
		t.Fatalf("grid size = %+v", g.Size)
	}
}

func TestHostReportsShellExit(t *testing.T) {
	h := newTestHost(t)
	if err := h.Spawn(7, domain.Size{W: 40, H: 10}, ""); err != nil {
		t.Fatal(err)
	}
	h.Paste(7, "exit\r")
	waitExited(t, h, 7)
}

func TestHostKillEndsShell(t *testing.T) {
	h := newTestHost(t)
	if err := h.Spawn(3, domain.Size{W: 40, H: 10}, ""); err != nil {
		t.Fatal(err)
	}
	h.Kill(3)
	waitExited(t, h, 3)
}

func TestHostSpawnFailure(t *testing.T) {
	h := NewHost(Options{Shell: "/nonexistent/shell"})
	t.Cleanup(h.Close)
	if err := h.Spawn(1, domain.Size{W: 10, H: 10}, ""); err == nil {
		t.Fatal("expected an error")
	}
}

func TestHostStartsInDirectoryAndReportsIt(t *testing.T) {
	h := newTestHost(t)
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	// Wide enough that the long temporary path does not wrap.
	if err := h.Spawn(1, domain.Size{W: 250, H: 10}, dir); err != nil {
		t.Fatal(err)
	}
	h.Paste(1, "pwd\r")
	waitForText(t, h, 1, dir)

	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0o755)
	h.Paste(1, "cd sub\r")
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, ok := h.WorkingDir(1)
		if ok && got == sub {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("working dir = %q, %v; want %q", got, ok, sub)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// A directory that no longer exists falls back to the default.
	if err := h.Spawn(2, domain.Size{W: 60, H: 10}, filepath.Join(dir, "gone")); err != nil {
		t.Fatal(err)
	}
}

func TestHostTitleFollowsForegroundProgramAndShellTitle(t *testing.T) {
	h := newTestHost(t)
	if err := h.Spawn(1, domain.Size{W: 60, H: 10}, ""); err != nil {
		t.Fatal(err)
	}
	waitTitle := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			select {
			case <-h.TitleChanged():
				if got, ok := h.TakeTitles()[1]; ok && got == want {
					return
				}
			case <-time.After(50 * time.Millisecond):
			}
			if time.Now().After(deadline) {
				t.Fatalf("title never became %q", want)
			}
		}
	}

	h.Paste(1, "sleep 30\r")
	waitTitle("sleep")
	h.SendKey(1, domain.Key{Code: 'c', Mod: domain.ModCtrl})
	waitTitle("sh")

	// A title set by the shell itself wins over the shell's name.
	h.Paste(1, "printf '\\033]2;my title\\007'\r")
	waitTitle("my title")
}

func TestHostScrollsIntoHistory(t *testing.T) {
	h := newTestHost(t)
	if err := h.Spawn(1, domain.Size{W: 40, H: 5}, ""); err != nil {
		t.Fatal(err)
	}
	h.Paste(1, "for i in $(seq 1 30); do echo line-$i; done\r")
	waitForText(t, h, 1, "line-30")

	st, _ := h.State(1)
	if st.Scrollback < 25 || st.AltScreen {
		t.Fatalf("state = %+v", st)
	}
	g, _ := h.Grid(1, 10)
	text := gridText(g)
	if !strings.Contains(text, "line-20") || strings.Contains(text, "line-30") {
		t.Fatalf("scrolled 10 lines up:\n%s", text)
	}
	if g.CursorVisible {
		t.Fatal("the cursor is hidden while scrolled up")
	}
	// Scrolling past the top stops at the oldest line: the command itself.
	g, _ = h.Grid(1, 99999)
	if first := strings.SplitN(gridText(g), "\n", 2)[0]; !strings.Contains(first, "for i in") {
		t.Fatalf("the top row should be the first line printed, got %q", first)
	}
}

func TestHostForwardsMouseOnlyWhenAskedFor(t *testing.T) {
	h := newTestHost(t)
	if err := h.Spawn(1, domain.Size{W: 60, H: 10}, ""); err != nil {
		t.Fatal(err)
	}
	click := domain.MouseEvent{Action: domain.MousePress, Button: domain.ButtonLeft, X: 4, Y: 2}
	if st, _ := h.State(1); st.Mouse != domain.MouseReportOff {
		t.Fatalf("a shell does not want the mouse, got %v", st.Mouse)
	}

	// A program turns on click reporting with SGR encoding, then echoes
	// what it reads.
	h.Paste(1, "printf '\\033[?1000h\\033[?1006h'; stty raw -echo; cat -v\r")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if st, _ := h.State(1); st.Mouse == domain.MouseReportClicks {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the program's mouse mode was never seen")
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond) // let cat start reading

	h.SendMouse(1, domain.MouseEvent{Action: domain.MouseMove, Button: domain.ButtonLeft, X: 9, Y: 9})
	h.SendMouse(1, click)
	waitForText(t, h, 1, "^[[<0;5;3M")
	if g, _ := h.Grid(1, 0); strings.Contains(gridText(g), ";10;10") {
		t.Fatal("click-only programs must not get motion")
	}
}
