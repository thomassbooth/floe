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

func TestTabsMarkActivityAndBells(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	// Window 1 prints and rings the bell after a while; window 2 has focus
	// by then.
	tm.typeLine(`sleep 1.5; echo done-later; sleep 1; printf '\a'`)
	tm.prefix("|")
	tm.waitFor(" 2 sh ")

	// Tab 1 is everything before tab 2; its title follows what is running,
	// like "1 sleep •".
	tab1 := func() string {
		bar := tm.lines()[29]
		return bar[:max(column(bar, "2 sh"), 0)]
	}
	tm.waitUntil("an activity dot on tab 1", func(string) bool { return strings.Contains(tab1(), "•") })
	tm.waitUntil("a bell mark on tab 1", func(string) bool { return strings.Contains(tab1(), "!") })
	if bar := tm.lines()[29]; strings.ContainsAny(bar[column(bar, "2 sh"):], "•!") {
		t.Fatalf("the focused window's tab must stay clean: %q", bar)
	}

	// The title catches up within half a second of the program ending.
	tm.waitUntil("tab 1 to read 1 sh again", func(string) bool { return strings.Contains(tab1(), "1 sh !") })
	tm.click(column(tm.lines()[29], "1 sh"), 29)
	tm.waitUntil("the marks to clear", func(string) bool {
		bar := tm.lines()[29]
		return !strings.Contains(bar, "!") && !strings.Contains(bar, "•")
	})
}
