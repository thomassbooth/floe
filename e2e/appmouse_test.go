package e2e

import (
	"strings"
	"testing"
)

// mouseApp starts a program in window 1 that turns on mouse reporting
// (button and drag events, SGR encoding) and prints what it receives, from
// the top of a cleared window. The window's title turning to "cat" means
// the program is running with its mouse mode on.
func mouseApp(t *testing.T) *term {
	tm := launch(t, 80, 24)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine(`clear; printf '\033[?1002h\033[?1006h'; stty raw -echo; cat -v`)
	tm.waitFor("╭─ 1 cat ")
	return tm
}

// appOutput is what window 1's program printed, with the window's borders
// and line wrapping taken out.
func (tm *term) appOutput() string {
	var b strings.Builder
	for _, l := range tm.lines()[1:22] { // the window's content rows
		b.WriteString(strings.TrimRight(strings.Trim(l, "│"), " "))
	}
	return b.String()
}

func (tm *term) waitOutput(want string) {
	tm.t.Helper()
	tm.waitUntil("the app to print "+want, func(string) bool { return strings.Contains(tm.appOutput(), want) })
}

func TestClicksAndWheelReachMouseApps(t *testing.T) {
	tm := mouseApp(t)
	// Window 1's content starts at (1, 1), so (6, 4) is (5, 3) inside it,
	// which SGR reports one-based as 6;4.
	tm.click(6, 4)
	tm.waitOutput("^[[<0;6;4M^[[<0;6;4m") // press and release

	tm.press(10, 5)
	tm.moveTo(12, 5)
	tm.release(12, 5)
	tm.waitOutput("^[[<0;10;5M^[[<32;12;5M^[[<0;12;5m") // a drag with the left button

	tm.wheel(20, 10, true)
	tm.waitOutput("^[[<64;20;10M")
	if strings.Contains(tm.screen(), "↑ ") {
		t.Fatal("the wheel went to the app, not to floe's scrollback")
	}
}

func TestArrangeModeKeepsTheMouseFromApps(t *testing.T) {
	tm := mouseApp(t)
	tm.arrange()
	tm.click(8, 6)
	tm.wheel(20, 10, true)
	tm.leaveArrange()

	tm.click(7, 4) // back in normal mode, the app gets clicks again
	tm.waitOutput("^[[<0;7;4m")
	if got := tm.appOutput(); got != "^[[<0;7;4M^[[<0;7;4m" {
		t.Fatalf("arrange mode leaked mouse events to the app: %q", got)
	}
}
