package terminal

import (
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
		g, ok := h.Grid(id)
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
	if _, ok := h.Grid(id); ok {
		t.Fatal("exited pane should be forgotten")
	}
}

func newTestHost() *Host {
	return NewHost(Options{Shell: "/bin/sh", Env: []string{"PS1=$ ", "ENV="}})
}

func TestHostRunsShellAndTypesKeys(t *testing.T) {
	h := newTestHost()
	if err := h.Spawn(1, domain.Size{W: 40, H: 10}); err != nil {
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
	h := newTestHost()
	if err := h.Spawn(1, domain.Size{W: 40, H: 10}); err != nil {
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
	h := newTestHost()
	if err := h.Spawn(7, domain.Size{W: 40, H: 10}); err != nil {
		t.Fatal(err)
	}
	h.Paste(7, "exit\r")
	waitExited(t, h, 7)
}

func TestHostKillEndsShell(t *testing.T) {
	h := newTestHost()
	if err := h.Spawn(3, domain.Size{W: 40, H: 10}); err != nil {
		t.Fatal(err)
	}
	h.Kill(3)
	waitExited(t, h, 3)
}

func TestHostSpawnFailure(t *testing.T) {
	h := NewHost(Options{Shell: "/nonexistent/shell"})
	if err := h.Spawn(1, domain.Size{W: 10, H: 10}); err == nil {
		t.Fatal("expected an error")
	}
}
