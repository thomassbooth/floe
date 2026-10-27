package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestADifferentBuildIsToldToReplaceTheServer(t *testing.T) {
	runtime := runtimeDir(t)
	tm := launchWith(t, options{w: 100, h: 30, runtime: runtime})
	tm.waitPrompts(1)

	// A rebuilt floe: same program, different bytes.
	raw, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "floe")
	if err := os.WriteFile(other, append(raw, 0), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(other)
	cmd.Env = tm.env
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), `is a different build of floe`) || !strings.Contains(string(out), `floe kill-server`) {
		t.Fatalf("got %v: %s", err, out)
	}
	// The running server and its client are untouched.
	tm.typeLine("echo still-$((1+1))")
	tm.waitFor("still-2")
}

func TestMuxRefusesToRunInsideItself(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitPrompts(1)
	tm.typeLine(binary + "; echo exit=$?")
	tm.waitFor("already inside floe")
	tm.waitFor("exit=1")
}

func TestACrashedServerIsReplaced(t *testing.T) {
	state, runtime := t.TempDir(), runtimeDir(t)
	tm := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	tm.waitFor(" 1 sh ")
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	waitSaved(t, state, 2)

	raw, err := os.ReadFile(filepath.Join(runtime, "floe", "default.lock"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	tm.waitFor("lost the connection to the floe server")
	tm.waitExit()
	if _, err := os.Stat(filepath.Join(runtime, "floe", "default.sock")); err != nil {
		t.Fatalf("expected the crashed server's socket to be left behind: %v", err)
	}

	again := launchWith(t, options{w: 100, h: 30, state: state, runtime: runtime})
	again.waitFor("╭─ 1 sh ")
	again.waitFor("╭─ 2 sh ")
}

// waitSaved waits until the sessions file in state lists n windows.
func waitSaved(t *testing.T, state string, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		raw, _ := os.ReadFile(filepath.Join(state, "floe", "sessions.json"))
		if strings.Count(string(raw), `"id":`) == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("sessions file never listed %d windows: %s", n, raw)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
