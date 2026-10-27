package e2e

import (
	"strings"
	"testing"
)

// centeredWindow starts floe at 100x30, opens window 2 in the middle (50x15
// at (25, 7)) and turns on arrange mode.
func centeredWindow(t *testing.T) *term {
	tm := centeredWindowNormal(t)
	tm.arrange()
	return tm
}

func TestDraggingToTheEdgeSnapsAndDraggingAwayKeepsTheSize(t *testing.T) {
	tm := centeredWindow(t)

	// Drag window 2 by its title to the middle of the left edge.
	tm.press(40, 7)
	tm.moveTo(20, 10)
	tm.moveTo(0, 14)
	// The left half is outlined while hovering there.
	tm.waitCell(0, 0, "╔")
	tm.waitCell(0, 28, "╚")
	tm.waitCell(49, 28, "╝")
	t.Logf("preview:\n%s", tm.screen())
	tm.release(0, 14)

	tm.waitUntil("window 2 to fill the left half", func(string) bool {
		return strings.HasPrefix(tm.lines()[0], "╭┄ 2 sh ")
	})
	tm.waitCell(49, 0, "╮")
	tm.waitCell(0, 28, "╰")

	// Dragging it away keeps its 50x29 snapped size. At full height it can
	// only slide sideways: grabbed 10 cells in, dropped at column 40.
	tm.press(10, 0)
	tm.moveTo(30, 5)
	tm.moveTo(40, 8)
	tm.release(40, 8)
	tm.waitCell(30, 0, "╭")
	tm.waitCell(79, 28, "╯")
}

func TestDraggingAMaximizedWindowRestoresItsSize(t *testing.T) {
	tm := centeredWindow(t)
	tm.send("z") // in arrange mode, z maximizes directly
	tm.waitUntil("window 2 to fill the screen", func(string) bool {
		return strings.HasPrefix(tm.lines()[0], "╭┄ 2 sh ")
	})

	// A maximized window cannot move at full size, so it comes away at its
	// 50x15 size from before; grabbed 20 cells into its 100-wide title, the
	// grab point lands 10 cells into the 50-wide one.
	tm.press(20, 0)
	tm.moveTo(30, 5)
	tm.moveTo(40, 8)
	tm.release(40, 8)
	tm.waitCell(30, 8, "╭")
	tm.waitCell(79, 22, "╯")
}

func TestDraggingAFullScreenWindowPopsItOut(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.arrange()

	// The only window fills the screen; dragging it shrinks it to half.
	tm.press(20, 0)
	tm.moveTo(35, 5)
	tm.moveTo(50, 10)
	tm.release(50, 10)
	tm.waitCell(40, 10, "╭")
	tm.waitCell(89, 23, "╯")
}

func TestSnapToCorner(t *testing.T) {
	tm := centeredWindow(t)
	tm.drag(40, 7, 99, 2) // top of the right edge
	tm.waitCell(50, 0, "╭")
	tm.waitCell(99, 14, "╯")
}

func TestPlainClickOnTitleDoesNotSnapOrMove(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.arrange()
	tm.click(0, 0) // top-left corner of a full-screen window, on the edge
	tm.click(20, 0)
	tm.leaveArrange()
	tm.typeLine("echo still-here")
	tm.waitFor("still-here")
	if l := tm.lines()[0]; !strings.HasPrefix(l, "╭─ 1 sh ") || !strings.HasSuffix(l, "╮") {
		t.Fatalf("window should still fill the screen: %q", l)
	}
}

func TestTidyFillsTheScreen(t *testing.T) {
	tm := centeredWindow(t)
	tm.send("c") // arrange mode keys work without the prefix
	tm.waitFor(" 3 sh ")

	tm.send("=")
	// Three windows: a left half and two right quarters, with no gaps.
	tm.waitCell(49, 0, "╮")
	tm.waitCell(50, 0, "╭")
	tm.waitCell(50, 15, "╭")
	tm.waitCell(99, 14, "╯")
	tm.waitCell(99, 28, "╯")
	tm.waitCell(0, 28, "╰")
	t.Logf("tidied:\n%s", tm.screen())
}

// jitterClick clicks the way some terminals report it: press, a motion
// report on the same cell, release.
func (tm *term) jitterClick(x, y int) {
	tm.t.Helper()
	tm.press(x, y)
	tm.moveTo(x, y)
	tm.release(x, y)
}

func TestClickingASnappedWindowLeavesItSnapped(t *testing.T) {
	tm := centeredWindow(t)
	tm.drag(40, 7, 0, 14)
	tm.waitCell(49, 28, "╯") // window 2 fills the left half

	tm.jitterClick(10, 0)
	tm.leaveArrange()
	tm.typeLine("echo after-click")
	tm.waitFor("after-click")
	if tm.cell(49, 28) != "╯" || !strings.HasPrefix(tm.lines()[0], "╭─ 2 sh ") {
		t.Fatalf("a click undid the snap:\n%s", tm.screen())
	}
}

func TestClickingAMaximizedWindowLeavesItMaximized(t *testing.T) {
	tm := centeredWindow(t)
	tm.send("z")
	tm.waitUntil("window 2 to fill the screen", func(string) bool {
		return strings.HasPrefix(tm.lines()[0], "╭┄ 2 sh ")
	})

	tm.jitterClick(20, 0)
	tm.leaveArrange()
	tm.typeLine("echo after-click")
	tm.waitFor("after-click")
	if !strings.HasPrefix(tm.lines()[0], "╭─ 2 sh ") {
		t.Fatalf("a click un-maximized the window:\n%s", tm.screen())
	}
}
