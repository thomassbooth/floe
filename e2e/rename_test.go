package e2e

import (
	"strings"
	"testing"
)

// renamed starts floe and names window 1 build.
func renamed(t *testing.T) *term {
	t.Helper()
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.prefix(",")
	tm.waitFor("rename window")
	tm.typeLine("build")
	tm.waitFor("╭─ 1 build ")
	return tm
}

func TestWindowNameWinsOverTheProgramsTitle(t *testing.T) {
	tm := renamed(t)
	if bar := tm.lines()[29]; !strings.Contains(bar, "1 build") {
		t.Fatalf("the tab should show the name: %q", bar)
	}

	// Neither a foreground program nor a title it sets changes the name.
	tm.typeLine(`printf '\033]2;from-program\007'; sleep 1; echo slept-$((1+1)); sleep 30`)
	tm.waitFor("slept-2")
	if s := tm.screen(); !strings.Contains(s, "╭─ 1 build ") || strings.Contains(s, "╭─ 1 sleep ") || strings.Contains(s, "╭─ 1 from-program ") {
		t.Fatalf("the name should stay:\n%s", s)
	}
	tm.send("\x03")
}

func TestClearingAWindowsNameBringsBackTheTitle(t *testing.T) {
	tm := renamed(t)

	// The prompt starts with the current name, so ctrl+u empties it.
	tm.prefix(",")
	tm.waitFor("rename window")
	tm.waitFor(" build ")
	tm.send("\x15\r")
	tm.waitFor("╭─ 1 sh ")

	tm.typeLine("sleep 30")
	tm.waitFor("╭─ 1 sleep ")
	tm.send("\x03")
}

func TestCancellingTheRenamePromptKeepsTheName(t *testing.T) {
	tm := renamed(t)
	tm.prefix(",")
	tm.waitFor("rename window")
	tm.send("-more")
	tm.waitFor(" build-more ")
	tm.send("\x1b")
	tm.waitFor("ctrl+b prefix")
	tm.waitFor("╭─ 1 build ")
}

func TestUnnamedWindowsPromptStartsEmpty(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.prefix(",")
	tm.waitFor("rename window")

	// Pressing enter straight away must not freeze the title as a name.
	tm.send("\r")
	tm.waitFor("ctrl+b prefix")
	tm.typeLine("sleep 30")
	tm.waitFor("╭─ 1 sleep ")
	tm.send("\x03")
}

func TestDoubleClickingATitleBarRenamesThatWindow(t *testing.T) {
	tm := twoWindows(t)
	x, y := tm.find("╭─ 1 sh ")
	tm.click(x+4, y)
	tm.click(x+4, y)
	tm.waitFor("rename window")
	tm.typeLine("left")
	tm.waitFor("╭─ 1 left ")
	if !strings.Contains(tm.screen(), "╭─ 2 sh ") {
		t.Fatalf("window 2 should keep its title:\n%s", tm.screen())
	}
}

func TestDoubleClickingATabRenamesThatWindow(t *testing.T) {
	tm := twoWindows(t)
	x := column(tm.lines()[29], "1 sh")
	tm.click(x, 29)
	tm.click(x, 29)
	tm.waitFor("rename window")
	tm.typeLine("left")
	tm.waitFor("╭─ 1 left ")
	tm.waitUntil("the tab to follow", func(string) bool { return strings.Contains(tm.lines()[29], "1 left") })
}

func TestOneClickOnATitleBarDoesNotRename(t *testing.T) {
	tm := twoWindows(t)
	x, y := tm.find("╭─ 1 sh ")
	tm.click(x+4, y)
	tm.waitUntil("window 1 to get focus", func(string) bool { return tm.focusedTab("1 sh") })
	if strings.Contains(tm.screen(), "rename window") {
		t.Fatalf("one click should not open the prompt:\n%s", tm.screen())
	}
}

func TestLongWindowNameLeavesRoomInTheStatusBar(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.prefix(",")
	tm.waitFor("rename window")
	tm.send(strings.Repeat("long ", 16))

	// The hint makes way for what is being typed instead of being cut off.
	tm.waitUntil("the hint to give way to the name", func(string) bool {
		bar := tm.lines()[29]
		return strings.Contains(bar, "long long long") && !strings.Contains(bar, "enter")
	})
	tm.send("\r")
	tm.waitFor("╭─ 1 long long ")
	tm.waitFor("ctrl+b prefix")

	bar := tm.lines()[29]
	if !strings.Contains(bar, "…") || strings.Count(bar, "long") > 6 {
		t.Fatalf("the tab should be cut short: %q", bar)
	}
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitUntil("the second tab to show", func(string) bool { return strings.Contains(tm.lines()[29], "2 sh") })

	// The cut tab still lands on its own window.
	tm.click(column(tm.lines()[29], "1 long"), 29)
	tm.waitUntil("window 1 to get focus", func(string) bool { return tm.focusedTab("1 long") })
}

func TestWhichKeyListsBothRenames(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.send("\x02")
	tm.waitFor(", ")
	tm.waitFor("rename window")
	s := tm.screen()
	if !strings.Contains(s, "$   rename") {
		t.Fatalf("the session rename should keep its own row:\n%s", s)
	}
}

func TestRenamedTabStillShowsBells(t *testing.T) {
	tm := renamed(t)
	tm.typeLine(`sleep 1.5; printf '\a'`)
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitUntil("a bell mark on the renamed tab", func(string) bool { return strings.Contains(tm.lines()[29], "1 build !") })
}

func TestWindowNameIsRememberedAfterARestart(t *testing.T) {
	state, runtime := t.TempDir(), runtimeDir(t)
	tm := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	tm.waitPrompts(1)
	tm.prefix(",")
	tm.waitFor("rename window")
	tm.typeLine("build")
	tm.waitFor("╭─ 1 build ")

	tm.killServer()
	tm.waitExit()

	again := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	again.waitFor("╭─ 1 build ")
}
