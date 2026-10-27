package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestClosingTheTerminalKeepsSessions(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "mx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	dir, _ = filepath.EvalSymlinks(dir)
	state := t.TempDir()

	tm := launchWith(t, options{w: 100, h: 30, state: state})
	tm.waitFor(" 1 sh ")
	tm.prefix("$")
	tm.waitFor("rename session")
	tm.send("\x15work\r")
	tm.waitBadge("work")
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)
	tm.typeLine("cd " + dir + " && echo moved")
	tm.waitFor("moved") // the shell has really changed directory

	// The terminal closes: mux gets SIGHUP and saves on the way out.
	tm.cmd.Process.Signal(syscall.SIGHUP)
	tm.waitExit()

	again := launchWith(t, options{w: 100, h: 30, state: state})
	again.waitBadge("work")
	again.waitFor("╭─ 1 sh ")
	again.waitFor("╭─ 2 sh ")
	if l := again.lines()[0]; column(l, "╭─ 2 sh") != 50 {
		t.Fatalf("window 2 should be back on the right half: %q", l)
	}
	again.waitPrompts(2)
	again.typeLine("pwd") // window 2 still has focus
	again.waitUntil("window 2's shell to start in "+dir, func(s string) bool {
		for _, l := range strings.Split(s, "\n") {
			if column(l, dir) > 50 {
				return true
			}
		}
		return false
	})
}

func TestKillingTheLastSessionStartsFreshNextTime(t *testing.T) {
	state := t.TempDir()
	tm := launchWith(t, options{w: 100, h: 30, state: state})
	tm.waitFor(" 1 sh ")
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.prefix("q")
	tm.waitExit()

	again := launchWith(t, options{w: 100, h: 30, state: state})
	again.waitFor(" 1 sh ")
	if strings.Contains(again.screen(), " 2 sh ") {
		t.Fatal("a killed session must not come back")
	}
}

func TestRestoredWindowsDoNotKeepOldProgramTitles(t *testing.T) {
	state := t.TempDir()
	tm := launchWith(t, options{w: 100, h: 30, state: state})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine("sleep 60")
	tm.waitFor("╭─ 1 sleep ")
	tm.cmd.Process.Signal(syscall.SIGHUP)
	tm.waitExit()

	// The program does not come back, so neither should its title.
	again := launchWith(t, options{w: 100, h: 30, state: state})
	again.waitPrompts(1)
	again.waitFor("╭─ 1 sh ")
}
