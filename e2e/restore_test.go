package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestClosingTheTerminalKeepsProgramsRunning(t *testing.T) {
	state, runtime := t.TempDir(), runtimeDir(t)
	tm := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	tm.waitPrompts(1)
	tm.typeLine("echo marker-$((6*7)); sleep 600")
	tm.waitFor("╭─ 1 sleep ")

	// The terminal closes: only the client goes.
	tm.cmd.Process.Signal(syscall.SIGHUP)
	tm.waitExit()

	again := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	again.waitFor("╭─ 1 sleep ")
	again.waitFor("marker-42")
	// It is the same program, still running: interrupt it.
	again.send("\x03")
	again.waitFor("╭─ 1 sh ")
}

func TestDetachKeyLeavesProgramsRunning(t *testing.T) {
	runtime := runtimeDir(t)
	tm := launchWith(t, options{w: 100, h: 30, runtime: runtime})
	tm.prefix("$")
	tm.waitFor("rename session")
	tm.send("\x15work\r")
	tm.waitBadge("work")
	tm.waitPrompts(1)
	tm.typeLine("sleep 600")
	tm.waitFor("╭─ 1 sleep ")

	tm.prefix("d")
	tm.waitExit()
	tm.waitFor("[detached from session work]")

	again := launchWith(t, options{w: 100, h: 30, runtime: runtime})
	again.waitBadge("work")
	again.waitFor("╭─ 1 sleep ")
}

func TestTwoTerminalsShowTheSameSession(t *testing.T) {
	runtime := runtimeDir(t)
	a := launchWith(t, options{w: 100, h: 30, runtime: runtime})
	a.waitPrompts(1)
	b := launchWith(t, options{w: 100, h: 30, runtime: runtime})
	b.waitPrompts(1)
	a.typeLine("echo from-$((1+1))-a")
	b.waitFor("from-2-a")
}

func TestKillServerSavesLayoutsForTheNextServer(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "mx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	dir, _ = filepath.EvalSymlinks(dir)
	state, runtime := t.TempDir(), runtimeDir(t)

	tm := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
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

	// Stopping the server detaches its clients.
	tm.killServer()
	tm.waitExit()

	again := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
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

func TestKillingTheLastSessionStopsTheServer(t *testing.T) {
	state, runtime := t.TempDir(), runtimeDir(t)
	tm := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	tm.waitFor(" 1 sh ")
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.prefix("q")
	tm.waitExit()
	waitServerGone(t, tm.env)

	again := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	again.waitFor(" 1 sh ")
	if strings.Contains(again.screen(), " 2 sh ") {
		t.Fatal("a killed session must not come back")
	}
}

func TestRestoredWindowsDoNotKeepOldProgramTitles(t *testing.T) {
	state, runtime := t.TempDir(), runtimeDir(t)
	tm := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine("sleep 60")
	tm.waitFor("╭─ 1 sleep ")
	tm.killServer()
	tm.waitExit()

	// The program does not come back, so neither should its title.
	again := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	again.waitPrompts(1)
	again.waitFor("╭─ 1 sh ")
}

// waitServerGone waits until no server runs for env.
func waitServerGone(t *testing.T, env []string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		cmd := exec.Command(binary, "kill-server")
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil && strings.Contains(string(out), "no mux server is running") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the server is still running: %s", out)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
