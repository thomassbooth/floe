package e2e

import (
	"strings"
	"testing"
)

func TestDraggingDoesNothingOutsideArrangeMode(t *testing.T) {
	tm := centeredWindowNormal(t)
	tm.drag(40, 7, 60, 12)
	tm.typeLine("echo still-here")
	tm.waitFor("still-here")
	if tm.cell(25, 7) != "╭" {
		t.Fatalf("window 2 moved without arrange mode:\n%s", tm.screen())
	}
}

func TestCtrlDragMovesAndResizesOutsideArrangeMode(t *testing.T) {
	tm := centeredWindowNormal(t) // window 2 is 50x15 at (25, 7)

	tm.ctrlDrag(40, 7, 50, 9) // by the title, ten right and two down
	tm.waitCell(35, 9, "╭")
	tm.ctrlDrag(84, 23, 89, 25) // by the bottom right corner
	tm.waitCell(89, 25, "╯")

	if strings.Contains(tm.screen(), "ARRANGE") {
		t.Fatal("ctrl+drag should not turn on arrange mode")
	}
	tm.typeLine("echo still-typing")
	tm.waitFor("still-typing")
}

func TestStatusBarButtonTogglesArrangeMode(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")

	bar := tm.lines()[29]
	x := column(bar, "arrange")
	if x < 0 {
		t.Fatalf("no arrange button in %q", bar)
	}
	tm.click(x, 29)
	tm.waitFor("ARRANGE")
	tm.waitCell(1, 0, "┄") // frames turn dashed

	// The lit-up badge is the button to leave again.
	tm.click(column(tm.lines()[29], "ARRANGE"), 29)
	tm.waitUntil("arrange mode to end", func(s string) bool { return !strings.Contains(s, "ARRANGE") })
	tm.waitCell(1, 0, "─")
}

func TestArrangeModeKeyboard(t *testing.T) {
	tm := centeredWindowNormal(t) // window 2 is 50x15 at (25, 7)
	tm.arrange()

	tm.send("ll") // two moves right, two columns each
	tm.waitCell(29, 7, "╭")
	tm.send("j")
	tm.waitCell(29, 8, "╭")
	tm.send("L") // two columns wider
	tm.waitCell(80, 8, "╮")

	// Typing does not reach the shell while arranging.
	tm.send("echo leaked\r")
	tm.send("\t") // switch to window 1, which comes to the front
	tm.waitUntil("window 1 on top", func(string) bool { return tm.cell(29, 8) != "╭" })
	if strings.Contains(tm.screen(), "leaked") {
		t.Fatal("keys reached the shell in arrange mode")
	}

	tm.leaveArrange()
	tm.typeLine("echo typed-again")
	tm.waitFor("typed-again")
}

func TestPrefixListShowsArrange(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.send("\x02")
	tm.waitFor("Sessions")
	if !strings.Contains(tm.screen(), "arrange") {
		t.Fatalf("command list should offer arrange:\n%s", tm.screen())
	}
}

// centeredWindowNormal is centeredWindow without turning on arrange mode.
func centeredWindowNormal(t *testing.T) *term {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.prefix("c")
	tm.waitFor(" 2 sh ")
	tm.waitCell(25, 7, "╭")
	tm.waitPrompts(2)
	return tm
}
