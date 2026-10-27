package e2e

import (
	"strings"
	"testing"
)

func TestMinimizeButtonHidesWindowUntilItsTabIsClicked(t *testing.T) {
	tm := twoWindows(t)
	tm.typeLine("sleep 1; echo still-running")

	// " _ □ × ─╮": the _ of window 2 is 7 cells left of its corner at 99.
	if got := tm.cell(92, 0); got != "_" {
		t.Fatalf("expected the minimize button at column 92:\n%s", tm.screen())
	}
	tm.press(92, 0)
	tm.release(92, 0)
	tm.waitUntil("window 2 to hide", func(s string) bool { return !strings.Contains(s, "╭─ 2 ") })
	if bar := tm.lines()[29]; !strings.Contains(bar, "2 sleep") && !strings.Contains(bar, "2 sh") {
		t.Fatalf("a minimized window keeps its tab: %q", bar)
	}
	// Window 1 has focus and keeps its half of the screen.
	tm.waitCell(49, 0, "╮")
	tm.typeLine("echo in-one")
	tm.waitFor("in-one")
	if c := column(tm.lines()[1], "in-one"); c > 49 {
		t.Fatalf("typing should go to window 1: %q", tm.lines()[1])
	}

	// Its program kept running while it was hidden.
	tm.waitUntil("tab 2 to read 2 sh", func(string) bool { return strings.Contains(tm.lines()[29], "2 sh") })
	tm.click(column(tm.lines()[29], "2 sh"), 29)
	tm.waitCell(50, 0, "╭")
	tm.waitFor("still-running")
	tm.waitFor("╭─ 2 sh ")
}

func TestMinimizeEveryWindowAndRestoreFromTheKeyboard(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)

	tm.prefix("m")
	tm.waitFor("Every window is minimized")
	tm.waitFor("ctrl+b M")
	if strings.Contains(tm.screen(), "╭─ 1 ") {
		t.Fatalf("window 1 should be hidden:\n%s", tm.screen())
	}

	tm.prefix("M")
	tm.waitFor("╭─ 1 sh ")
	tm.typeLine("echo back-again")
	tm.waitFor("back-again")
	if strings.Contains(tm.screen(), "Every window is minimized") {
		t.Fatalf("the hint should go once a window is back:\n%s", tm.screen())
	}

	// With every window minimized, clicking a tab brings that one back.
	tm.prefix("m")
	tm.waitFor("Every window is minimized")
	tm.click(column(tm.lines()[29], "1 sh"), 29)
	tm.waitFor("╭─ 1 sh ")
	tm.typeLine("echo clicked-back")
	tm.waitFor("clicked-back")
}
