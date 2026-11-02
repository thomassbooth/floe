package e2e

import (
	"fmt"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

// attrsAt returns the text attributes of the cell at x, y.
func (tm *term) attrsAt(x, y int) uint8 {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if c := tm.emu.CellAt(x, y); c != nil {
		return c.Style.Attrs
	}
	return 0
}

// locate returns where text first appears on screen, if it does.
func (tm *term) locate(text string) (x, y int, ok bool) {
	for y, line := range tm.lines() {
		if x := column(line, text); x >= 0 {
			return x, y, true
		}
	}
	return 0, 0, false
}

// find returns where text first appears on screen, which must be somewhere.
func (tm *term) find(text string) (x, y int) {
	tm.t.Helper()
	x, y, ok := tm.locate(text)
	if !ok {
		tm.t.Fatalf("%q is not on screen:\n%s", text, tm.screen())
	}
	return x, y
}

// waitFaint waits for the first cell of text to be drawn faint, or not.
func (tm *term) waitFaint(text string, faint bool) {
	tm.t.Helper()
	tm.waitUntil(fmt.Sprintf("%q to have faint = %v", text, faint), func(string) bool {
		x, y := tm.find(text)
		return tm.attrsAt(x, y)&uv.AttrFaint != 0 == faint
	})
}

func TestUnfocusedWindowsAreDimmed(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitPrompts(1)
	tm.typeLine("echo left-$((1+1))")
	tm.waitFor("left-2")
	tm.waitFaint("left-2", false)

	// Splitting moves focus to the new window, so the old one dims.
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)
	tm.waitFaint("left-2", true)
	tm.typeLine("echo right-$((1+1))")
	tm.waitFor("right-2")
	tm.waitFaint("right-2", false)

	// Dimming follows focus.
	x, y := tm.find("left-2")
	tm.click(x, y)
	tm.waitFaint("left-2", false)
	tm.waitFaint("right-2", true)
}

func TestDimmingKeepsAProgramsColorsAndOverridesBold(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitPrompts(1)
	tm.typeLine(`printf '\033[1;38;2;10;200;30mgreen-%s\033[0m\n' bold`)
	tm.waitFor("green-bold")
	x, y := tm.find("green-bold")
	before := tm.fg(x, y)
	if tm.attrsAt(x, y)&uv.AttrBold == 0 {
		t.Fatalf("the focused window should show the program's bold")
	}

	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitFaint("green-bold", true)
	if got := tm.fg(x, y); !sameColor(got, before) {
		t.Fatalf("dimming changed the program's color from %v to %v", before, got)
	}
	// Terminals treat bold and faint together differently, and some make
	// the text brighter, so a dimmed cell is only faint.
	if tm.attrsAt(x, y)&uv.AttrBold != 0 {
		t.Fatalf("a dimmed cell should not stay bold")
	}
}

func TestDimmingCanBeTurnedOff(t *testing.T) {
	config := t.TempDir()
	writeConfig(t, config, "animations off\ndim-unfocused off\n")
	tm := launchWith(t, options{w: 100, h: 30, config: config, animate: true})
	tm.waitPrompts(1)
	tm.typeLine("echo left-$((1+1))")
	tm.waitFor("left-2")

	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)
	tm.typeLine("echo right-$((1+1))")
	tm.waitFor("right-2")
	if x, y := tm.find("left-2"); tm.attrsAt(x, y)&uv.AttrFaint != 0 {
		t.Fatalf("dimming is off, but the window is faint:\n%s", tm.screen())
	}
	if strings.Contains(tm.screen(), "floe.conf") {
		t.Fatalf("the setting should be accepted:\n%s", tm.screen())
	}
}
