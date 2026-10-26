package e2e

import (
	"fmt"
	"strings"
	"testing"
)

// press holds the left button down at x, y.
func (tm *term) press(x, y int) {
	tm.t.Helper()
	tm.send(fmt.Sprintf("\x1b[<0;%d;%dM", x+1, y+1))
}

// moveTo moves the mouse with the left button held.
func (tm *term) moveTo(x, y int) {
	tm.t.Helper()
	tm.send(fmt.Sprintf("\x1b[<32;%d;%dM", x+1, y+1))
}

// release lets go of the left button at x, y.
func (tm *term) release(x, y int) {
	tm.t.Helper()
	tm.send(fmt.Sprintf("\x1b[<0;%d;%dm", x+1, y+1))
}

// drag presses at one cell, moves to another in a few steps and releases.
func (tm *term) drag(fromX, fromY, toX, toY int) {
	tm.t.Helper()
	tm.press(fromX, fromY)
	for i := 1; i <= 3; i++ {
		tm.moveTo(fromX+(toX-fromX)*i/3, fromY+(toY-fromY)*i/3)
	}
	tm.release(toX, toY)
}

// cell returns the character drawn at x, y.
func (tm *term) cell(x, y int) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if c := tm.emu.CellAt(x, y); c != nil {
		return c.Content
	}
	return ""
}

func (tm *term) waitCell(x, y int, want string) {
	tm.t.Helper()
	tm.waitUntil(fmt.Sprintf("%q at (%d,%d)", want, x, y), func(string) bool { return tm.cell(x, y) == want })
}

// twoWindows starts mux at 100x30 with window 1 on the left half and
// window 2 on the right half, focused.
func twoWindows(t *testing.T) *term {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)
	tm.waitCell(50, 0, "╭")
	return tm
}

func TestDragTitleMovesWindow(t *testing.T) {
	tm := twoWindows(t)
	// Window 2 is full height, so shorten it first to give it room to move
	// down: drag its bottom edge from row 28 up to row 18.
	tm.drag(75, 28, 75, 18)
	tm.waitCell(50, 18, "╰")

	// Grab window 2's title 10 cells in and drop it at (30, 5).
	tm.press(60, 0)
	tm.moveTo(45, 3)
	tm.waitCell(35, 3, "┏") // heavy border while dragging
	tm.moveTo(30, 5)
	tm.release(30, 5)

	tm.waitCell(20, 5, "╭")
	if l := tm.lines()[5]; column(l, "╭─ 2 sh ") != 20 {
		t.Fatalf("window 2 should start at column 20: %q", l)
	}
	t.Logf("after move:\n%s", tm.screen())
}

func TestDragWindowStaysOnScreen(t *testing.T) {
	tm := twoWindows(t)
	tm.drag(60, 0, 99, 29)
	// Window 2 is 50x29, so it cannot move at all down or right.
	tm.waitCell(50, 0, "╭")
	tm.drag(60, 0, 0, 0)
	tm.waitUntil("window 2 at the left edge", func(string) bool {
		return strings.HasPrefix(tm.lines()[0], "╭─ 2 sh ")
	})
}

func TestDragCornerResizesWindow(t *testing.T) {
	tm := twoWindows(t)
	// Move window 2 left by 20; at full height it stays on row 0.
	tm.drag(60, 0, 40, 2)
	tm.waitCell(30, 0, "╭")
	// Drag its bottom-right corner (79, 28) in by 10 and up by 10.
	tm.drag(79, 28, 69, 18)
	tm.waitCell(69, 18, "╯")

	tm.typeLine("stty size")
	tm.waitFor("17 38") // 40x19 frame -> 38x17 terminal
}

func TestCloseButton(t *testing.T) {
	tm := twoWindows(t)
	// " □ × ─╮": the × of window 2 is 3 cells left of its corner at 99.
	tm.press(96, 0)
	tm.release(96, 0)
	tm.waitUntil("window 2 to close", func(s string) bool { return !strings.Contains(s, " 2 sh ") })
	// Window 1 takes the space back.
	tm.waitCell(99, 0, "╮")
}

func TestMaximizeButton(t *testing.T) {
	tm := twoWindows(t)
	tm.press(94, 0) // the □ of window 2
	tm.release(94, 0)
	tm.waitUntil("window 2 to fill the screen", func(string) bool {
		return strings.HasPrefix(tm.lines()[0], "╭─ 2 sh ")
	})
	tm.press(94, 0)
	tm.release(94, 0)
	tm.waitCell(50, 0, "╭")
}

func TestClickBringsWindowToFront(t *testing.T) {
	tm := twoWindows(t)
	// Drag window 2 so it overlaps window 1, then click the part of window
	// 1 still showing.
	tm.drag(60, 0, 40, 0) // window 2 now starts at column 30
	tm.waitCell(30, 0, "╭")

	tm.press(5, 5)
	tm.release(5, 5)
	// Window 1 is on top: its right border at column 49 is drawn over
	// window 2.
	tm.waitCell(49, 5, "│")
	if got := tm.cell(49, 0); got != "╮" {
		t.Fatalf("window 1's corner should be on top, got %q", got)
	}
}
