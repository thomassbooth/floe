package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wheel turns the mouse wheel one notch at x, y.
func (tm *term) wheel(x, y int, up bool) {
	tm.t.Helper()
	b := 65
	if up {
		b = 64
	}
	tm.send(fmt.Sprintf("\x1b[<%d;%d;%dM", b, x+1, y+1))
}

func TestWheelScrollsHistoryAndTypingReturns(t *testing.T) {
	tm := launch(t, 80, 24)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine("for i in $(seq 1 100); do echo line-$i; done")
	tm.waitFor("line-100")

	for range 10 {
		tm.wheel(20, 10, true)
	}
	tm.waitFor("↑ 30")
	// 22 rows of window, 30 lines up: lines 51 to 71 show.
	tm.waitUntil("older lines", func(s string) bool {
		return strings.Contains(s, "line-51") && strings.Contains(s, "line-71") && !strings.Contains(s, "line-72")
	})
	t.Logf("scrolled:\n%s", tm.screen())

	tm.wheel(20, 10, false)
	tm.waitFor("↑ 27")

	tm.send("e")
	tm.waitUntil("back at the bottom", func(s string) bool {
		return !strings.Contains(s, "↑ ") && strings.Contains(s, "line-100")
	})
}

func TestScrolledViewHoldsStillDuringOutput(t *testing.T) {
	tm := launch(t, 80, 24)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	// The ticks wait for the go file, so the view is scrolled to a known
	// place before any of them arrive. Each one then raises the marker.
	start := filepath.Join(t.TempDir(), "go")
	tm.typeLine(fmt.Sprintf("seq 1 60; echo ready; ( (while [ ! -e %s ]; do sleep 0.02; done; for i in $(seq 1 40); do echo tick-$i; sleep 0.02; done) &)", start))
	tm.waitUntil("the prompt after ready", func(string) bool {
		lines := tm.lines()
		for i := range len(lines) - 1 {
			if strings.HasPrefix(lines[i], "│ready ") && strings.Contains(lines[i+1], "$ ") {
				return true
			}
		}
		return false
	})

	for range 5 {
		tm.wheel(20, 10, true)
	}
	tm.waitFor("↑ 15")
	before := tm.lines()[5]
	if err := os.WriteFile(start, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tm.waitFor("↑ 55")
	if after := tm.lines()[5]; after != before {
		t.Fatalf("view moved while scrolled:\nbefore %q\nafter  %q", before, after)
	}
}

func TestWheelInAFullScreenProgramSendsArrows(t *testing.T) {
	tm := launch(t, 80, 24)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine("seq 1 200 | less")
	tm.waitUntil("less to show the start", func(string) bool {
		return strings.TrimSpace(strings.Trim(tm.lines()[1], "│")) == "1"
	})

	tm.wheel(20, 10, false) // three lines down
	tm.waitUntil("less to scroll", func(string) bool {
		return strings.TrimSpace(strings.Trim(tm.lines()[1], "│")) == "4"
	})
	if strings.Contains(tm.screen(), "↑ ") {
		t.Fatal("a full-screen program has no floe scrollback")
	}
	tm.send("q")
}
