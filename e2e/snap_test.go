package e2e

import (
	"strings"
	"testing"
)

// centeredWindow starts mux at 100x30 and opens window 2 in the middle:
// 50x15 at (25, 7).
func centeredWindow(t *testing.T) *term {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.prefix("c")
	tm.waitFor(" 2 sh ")
	tm.waitCell(25, 7, "╭")
	return tm
}

func TestDraggingToTheEdgeSnapsAndDraggingAwayRestores(t *testing.T) {
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
		return strings.HasPrefix(tm.lines()[0], "╭─ 2 sh ")
	})
	tm.waitCell(49, 0, "╮")
	tm.waitCell(0, 28, "╰")

	// Dragging it away gives it back its 50x15 size; the grab point stays
	// 10 cells along the title.
	tm.press(10, 0)
	tm.moveTo(30, 5)
	tm.moveTo(40, 8)
	tm.release(40, 8)
	tm.waitCell(30, 8, "╭")
	tm.waitCell(79, 22, "╯")
}

func TestDraggingAFullScreenWindowPopsItOut(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")

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
	tm.click(0, 0) // top-left corner of a full-screen window, on the edge
	tm.click(20, 0)
	tm.typeLine("echo still-here")
	tm.waitFor("still-here")
	if l := tm.lines()[0]; !strings.HasPrefix(l, "╭─ 1 sh ") || !strings.HasSuffix(l, "╮") {
		t.Fatalf("window should still fill the screen: %q", l)
	}
}

func TestTidyFillsTheScreen(t *testing.T) {
	tm := centeredWindow(t)
	tm.prefix("c")
	tm.waitFor(" 3 sh ")

	tm.prefix("=")
	// Three windows: a left half and two right quarters, with no gaps.
	tm.waitCell(49, 0, "╮")
	tm.waitCell(50, 0, "╭")
	tm.waitCell(50, 15, "╭")
	tm.waitCell(99, 14, "╯")
	tm.waitCell(99, 28, "╯")
	tm.waitCell(0, 28, "╰")
	t.Logf("tidied:\n%s", tm.screen())
}
