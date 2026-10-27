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
