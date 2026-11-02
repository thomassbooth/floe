package e2e

import (
	"fmt"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

// cursorHidden reports whether the terminal's cursor is hidden.
func (tm *term) cursorHidden() bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.emu.CursorHidden()
}

// focusedTab reports whether the tab labeled label is the focused one, which
// the status bar draws in bold.
func (tm *term) focusedTab(label string) bool {
	x := column(tm.lines()[29], label)
	return x >= 0 && tm.attrsAt(x, 29)&uv.AttrBold != 0
}

// pinnedOverOne opens a small window over a full-screen window 1 and pins
// it, leaving it focused. It returns where the pinned window's title is.
func pinnedOverOne(t *testing.T) (tm *term, x, y int) {
	t.Helper()
	tm = launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.prefix("c")
	tm.waitFor("╭─ 2 sh ")
	tm.waitPrompts(2)
	tm.prefix("p")
	tm.waitFor("╭─ 2 ◆ sh ")
	x, y = tm.find("╭─ 2 ◆ sh ")
	return tm, x, y
}

func TestPinnedWindowStaysOnTopOfTheFocusedOne(t *testing.T) {
	tm, x, y := pinnedOverOne(t)

	// Focusing window 1 would normally bring it over window 2.
	tm.click(column(tm.lines()[29], "1 sh"), 29)
	tm.typeLine("echo typed-$((1+1))")
	tm.waitFor("typed-2")
	if lx, _ := tm.find("typed-2"); lx >= x {
		t.Fatalf("typing should go to window 1:\n%s", tm.screen())
	}
	if got, gy := tm.find("╭─ 2 ◆ sh "); got != x || gy != y {
		t.Fatalf("window 2 should stay where it was, on top:\n%s", tm.screen())
	}

	// Clicking the pinned window gets to it, even though it has lost focus.
	tm.click(x+5, y+3)
	tm.typeLine("echo pinned-$((1+1))")
	tm.waitFor("pinned-2")
	if lx, _ := tm.find("pinned-2"); lx <= x {
		t.Fatalf("typing should go to window 2:\n%s", tm.screen())
	}
}

func TestPinnedWindowFloatsOverAMaximizedOne(t *testing.T) {
	tm := twoWindows(t)
	tm.prefix("c")
	tm.waitFor("╭─ 3 sh ")
	tm.waitPrompts(3)
	tm.prefix("p")
	tm.waitFor("╭─ 3 ◆ sh ")
	tm.click(column(tm.lines()[29], "2 sh"), 29)
	tm.waitUntil("window 2 to get focus", func(string) bool { return tm.focusedTab("2 sh") })

	tm.prefix("z")
	tm.waitFor("╭─ 2 sh ")
	tm.waitCell(0, 0, "╭")
	if !strings.Contains(tm.screen(), "╭─ 3 ◆ sh ") {
		t.Fatalf("the pinned window should stay in view:\n%s", tm.screen())
	}
}

func TestMaximizedPinnedWindowLetsOthersBeSeen(t *testing.T) {
	tm, _, _ := pinnedOverOne(t)
	tm.prefix("z")
	tm.waitCell(0, 0, "╭")
	tm.waitFor("╭─ 2 ◆ sh ")

	// Otherwise it would hide window 1 for as long as it stays maximized.
	tm.click(column(tm.lines()[29], "1 sh"), 29)
	tm.waitFor("╭─ 1 sh ")
}

func TestUnpinningPutsAWindowBackInTheStack(t *testing.T) {
	tm, _, _ := pinnedOverOne(t)

	tm.prefix("p")
	tm.waitFor("╭─ 2 sh ")
	if strings.Contains(tm.lines()[29], "◆") {
		t.Fatalf("the tab should lose its mark: %q", tm.lines()[29])
	}
	tm.click(column(tm.lines()[29], "1 sh"), 29)
	tm.waitUntil("window 1 to cover window 2", func(s string) bool { return !strings.Contains(s, "╭─ 2 ") })
}

func TestPinnedWindowShowsOnItsTab(t *testing.T) {
	tm, _, _ := pinnedOverOne(t)
	if bar := tm.lines()[29]; !strings.Contains(bar, "2 ◆ sh") {
		t.Fatalf("the tab should carry the mark: %q", bar)
	}
	// The tab still lands on the window it is drawn for.
	tm.click(column(tm.lines()[29], "1 sh"), 29)
	tm.waitUntil("window 1 to get focus", func(string) bool { return tm.focusedTab("1 sh") })
	tm.click(column(tm.lines()[29], "2 ◆ sh"), 29)
	tm.waitUntil("window 2 to get focus", func(string) bool { return tm.focusedTab("2 ◆ sh") })
	tm.typeLine("echo in-two-$((1+1))")
	tm.waitFor("in-two-2")
}

func TestCursorHidesBehindAPinnedWindow(t *testing.T) {
	tm, _, y := pinnedOverOne(t)
	tm.click(column(tm.lines()[29], "1 sh"), 29)

	tm.waitUntil("window 1 to get focus", func(string) bool { return tm.focusedTab("1 sh") })
	if tm.cursorHidden() {
		t.Fatalf("window 1's cursor should show while it is clear of window 2:\n%s", tm.screen())
	}

	// Print until window 1's prompt, and so its cursor, is under window 2.
	tm.typeLine(fmt.Sprintf("clear; i=0; while [ $i -lt %d ]; do echo; i=$((i+1)); done; printf '%%50s' ''", y+3))
	tm.waitUntil("the cursor to hide under window 2", func(string) bool { return tm.cursorHidden() })

	// It comes back once window 1 is drawn over window 2.
	tm.click(column(tm.lines()[29], "2 ◆ sh"), 29)
	tm.waitUntil("window 2 to get focus", func(string) bool { return tm.focusedTab("2 ◆ sh") })
	tm.prefix("p")
	tm.waitFor("╭─ 2 sh ")
	tm.click(column(tm.lines()[29], "1 sh"), 29)
	tm.waitUntil("the cursor to come back", func(string) bool { return !tm.cursorHidden() })
}

func TestTidyLeavesAPinnedWindowWhereItIs(t *testing.T) {
	tm := twoWindows(t)
	tm.prefix("c")
	tm.waitFor("╭─ 3 sh ")
	tm.waitPrompts(3)
	tm.prefix("p")
	tm.waitFor("╭─ 3 ◆ sh ")
	x, y := tm.find("╭─ 3 ◆ sh ")

	tm.prefix("=")
	tm.typeLine("echo tidied-$((1+1))")
	tm.waitFor("tidied-2")
	if got, gy := tm.find("╭─ 3 ◆ sh "); got != x || gy != y {
		t.Fatalf("the pinned window should not move:\n%s", tm.screen())
	}
	// The other two keep sharing the whole screen, underneath it.
	tm.waitCell(50, 0, "╭")
	tm.waitCell(99, 0, "╮")
}

func TestPinnedWindowMovesInArrangeMode(t *testing.T) {
	tm, x, y := pinnedOverOne(t)

	tm.prefix("a")
	tm.waitFor("ARRANGE")
	tm.send("l")
	tm.send("j")
	tm.waitUntil("window 2 to move", func(string) bool {
		gx, gy, ok := tm.locate("╭┄ 2 ◆ sh ")
		return ok && gx == x+2 && gy == y+1
	})

	// Arrange mode reaches it from another window with tab, and can pin
	// and unpin from there.
	tm.send("\t")
	tm.waitUntil("window 1 to get focus", func(string) bool { return tm.focusedTab("1 sh") })
	tm.send("\t")
	tm.waitUntil("window 2 to get focus", func(string) bool { return tm.focusedTab("2 ◆ sh") })
	tm.send("p")
	tm.waitFor("╭┄ 2 sh ")
}

func TestPinIsRememberedAfterARestart(t *testing.T) {
	state, runtime := t.TempDir(), runtimeDir(t)
	tm := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	tm.waitPrompts(1)
	tm.prefix("c")
	tm.waitFor("╭─ 2 sh ")
	tm.waitPrompts(2)
	tm.prefix("p")
	tm.waitFor("╭─ 2 ◆ sh ")
	tm.click(column(tm.lines()[29], "1 sh"), 29)
	tm.waitUntil("window 1 to get focus", func(string) bool { return tm.focusedTab("1 sh") })

	tm.killServer()
	tm.waitExit()

	again := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	again.waitFor("╭─ 2 ◆ sh ")
	again.waitUntil("window 1 to keep focus", func(string) bool { return again.focusedTab("1 sh") })
}
