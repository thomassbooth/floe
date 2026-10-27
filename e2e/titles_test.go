package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWindowsOpenInTheFocusedWindowsDirectory(t *testing.T) {
	// A short path, so pwd's output fits on one line of a half-width window.
	dir, err := os.MkdirTemp("/tmp", "mx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	dir, _ = filepath.EvalSymlinks(dir)
	work := filepath.Join(dir, "work")
	os.Mkdir(work, 0o755)

	tm := launch(t, 160, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine("cd " + work)

	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)
	tm.typeLine("pwd")
	tm.waitUntil("window 2 to start in "+work, func(s string) bool {
		for _, l := range strings.Split(s, "\n") {
			if c := column(l, work); c > 80 {
				return true
			}
		}
		return false
	})
}

func TestTitleShowsTheRunningProgram(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)

	tm.typeLine("sleep 30")
	tm.waitFor("╭─ 1 sleep ")
	if bar := tm.lines()[29]; !strings.Contains(bar, "1 sleep") {
		t.Fatalf("the status bar tab should show the program: %q", bar)
	}

	tm.send("\x03") // ctrl+c ends it; the shell is back
	tm.waitFor("╭─ 1 sh ")
}

func TestTabsShowBusyPanesAndBells(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	bar := func() string { return tm.lines()[29] }

	// A program that keeps printing marks its tab as busy, even while its
	// window has focus.
	tm.typeLine(`sleep 1; while :; do echo tick; sleep 0.2; done`)
	tm.waitUntil("a busy dot on the focused tab", func(string) bool { return strings.Contains(bar(), "•") })

	// Once it stops printing, the dot goes.
	tm.send("\x03")
	tm.waitUntil("the dot to clear when the pane goes quiet", func(string) bool { return !strings.Contains(bar(), "•") })

	// Window 1 rings the bell once window 2 has focus.
	tm.typeLine(`sleep 1.5; printf '\a'`)
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tab1 := func() string { return bar()[:max(column(bar(), "2 sh"), 0)] }
	tm.waitUntil("a bell mark on tab 1", func(string) bool { return strings.Contains(tab1(), "!") })
	if b := bar(); strings.ContainsAny(b[column(b, "2 sh"):], "•!") {
		t.Fatalf("an idle window's tab must stay clean: %q", b)
	}

	// Looking at the window clears the bell. Its title catches up within
	// half a second of the program ending.
	tm.waitUntil("tab 1 to read 1 sh again", func(string) bool { return strings.Contains(tab1(), "1 sh !") })
	tm.click(column(bar(), "1 sh"), 29)
	tm.waitUntil("the bell to clear", func(string) bool { return !strings.Contains(bar(), "!") })
}
