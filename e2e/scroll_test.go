package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"
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
	tm.typeLine("seq 1 60; (for i in $(seq 1 40); do echo tick-$i; sleep 0.05; done &)")
	tm.waitFor("tick-3")

	for range 5 {
		tm.wheel(20, 10, true)
	}
	tm.waitFor("↑ 15")
	before := tm.lines()[5]
	// The ticks keep printing for about two more seconds, out of sight.
	time.Sleep(2500 * time.Millisecond)
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
