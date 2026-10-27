// Package e2e runs the real floe binary in a pseudo-terminal and checks what
// a user would see on screen.
package e2e

import (
	"encoding/base64"
	"fmt"
	"image/color"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "floe-e2e")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "floe")
	build := exec.Command("go", "build", "-o", binary, "../cmd/floe")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// term is an outer terminal running floe.
type term struct {
	t   *testing.T
	cmd *exec.Cmd
	pty *os.File
	// env is floe's environment, which also finds its server.
	env  []string
	mu   sync.Mutex
	emu  *vt.Emulator
	done chan struct{}
	// clipboard is what floe last copied to the outer terminal's clipboard.
	clipboard string
}

func launch(t *testing.T, w, h int) *term {
	t.Helper()
	return launchShell(t, w, h, "/bin/sh")
}

func launchShell(t *testing.T, w, h int, shell string) *term {
	t.Helper()
	return launchWith(t, options{w: w, h: h, shell: shell})
}

// options describes how to start floe.
type options struct {
	w, h  int
	shell string
	// config and state are used as XDG_CONFIG_HOME and XDG_STATE_HOME, so
	// tests never touch the real settings or saved sessions. Reusing a
	// directory carries them across runs.
	config, state string
	// runtime is used as XDG_RUNTIME_DIR, where the server's socket lives.
	// Each test gets its own server; reusing a directory attaches to the
	// same one.
	runtime string
	// binary, when set, is the floe binary to run instead of the one built
	// for the tests.
	binary string
	// background, when set, is the outer terminal's background color, as
	// reported to floe when it asks.
	background color.Color
}

func launchWith(t *testing.T, o options) *term {
	t.Helper()
	w, h, shell := o.w, o.h, o.shell
	if shell == "" {
		shell = "/bin/sh"
	}
	if o.config == "" {
		o.config = t.TempDir()
	}
	if o.state == "" {
		o.state = t.TempDir()
	}
	if o.runtime == "" {
		o.runtime = runtimeDir(t)
	}
	if testing.Short() {
		t.Skip("e2e tests are slow")
	}
	// Describe the outer terminal as what the tests emulate, a true color
	// xterm, whatever terminal the tests themselves run in. The tests may
	// also run inside floe.
	env := slices.DeleteFunc(os.Environ(), func(v string) bool {
		name, _, _ := strings.Cut(v, "=")
		return name == "FLOE" || name == "COLORTERM" || name == "NO_COLOR" ||
			strings.HasPrefix(name, "TERM") || strings.HasPrefix(name, "CLICOLOR")
	})
	env = append(env,
		"SHELL="+shell,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"XDG_CONFIG_HOME="+o.config,
		"XDG_STATE_HOME="+o.state,
		"XDG_RUNTIME_DIR="+o.runtime,
	)
	if o.binary == "" {
		o.binary = binary
	}
	cmd := exec.Command(o.binary)
	cmd.Env = env
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(w), Rows: uint16(h)})
	if err != nil {
		t.Fatal(err)
	}
	tm := &term{t: t, cmd: cmd, env: env, pty: f, emu: vt.NewEmulator(w, h), done: make(chan struct{})}
	if o.background != nil {
		tm.emu.SetDefaultBackgroundColor(o.background)
	}
	// Play the outer terminal's clipboard: keep what floe copies with OSC 52.
	tm.emu.RegisterOscHandler(52, func(data []byte) bool {
		parts := strings.SplitN(string(data), ";", 3)
		if len(parts) == 3 {
			if text, err := base64.StdEncoding.DecodeString(parts[2]); err == nil {
				tm.clipboard = string(text) // runs inside Write, under tm.mu
			}
		}
		return true
	})

	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				tm.mu.Lock()
				tm.emu.Write(buf[:n])
				tm.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	// Answer the terminal queries Bubble Tea sends at startup.
	go io.Copy(f, tm.emu)
	go func() {
		cmd.Wait()
		close(tm.done)
	}()

	t.Cleanup(func() {
		cmd.Process.Kill()
		<-tm.done
		f.Close()
		// Stop this test's server too, unless it already stopped. Never
		// without env: that would find the user's own server.
		kill := exec.Command(binary, "kill-server")
		kill.Env = env
		_ = kill.Run()
	})
	return tm
}

// runtimeDir returns a fresh directory for a server's socket. Socket paths
// are short on macOS, so it avoids the long t.TempDir.
func runtimeDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// killServer runs "floe kill-server" for this terminal's server.
func (tm *term) killServer() {
	tm.t.Helper()
	cmd := exec.Command(binary, "kill-server")
	cmd.Env = tm.env
	if out, err := cmd.CombinedOutput(); err != nil {
		tm.t.Fatalf("kill-server: %v: %s", err, out)
	}
}

func (tm *term) send(s string) {
	tm.t.Helper()
	if _, err := tm.pty.WriteString(s); err != nil {
		tm.t.Fatal(err)
	}
}

// typeLine types a shell command and presses enter.
func (tm *term) typeLine(s string) {
	tm.t.Helper()
	tm.send(s + "\r")
}

// prefix presses ctrl+b followed by key.
func (tm *term) prefix(key string) {
	tm.t.Helper()
	tm.send("\x02")
	time.Sleep(50 * time.Millisecond)
	tm.send(key)
}

func (tm *term) click(x, y int) {
	tm.t.Helper()
	tm.send(fmt.Sprintf("\x1b[<0;%d;%dM\x1b[<0;%d;%dm", x+1, y+1, x+1, y+1))
}

func (tm *term) screen() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	var b strings.Builder
	for y := range tm.emu.Height() {
		for x := 0; x < tm.emu.Width(); {
			c := tm.emu.CellAt(x, y)
			if c == nil || c.Content == "" {
				b.WriteByte(' ')
				x++
				continue
			}
			b.WriteString(c.Content)
			x += max(c.Width, 1)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// fg returns the foreground color of the cell at x, y.
func (tm *term) fg(x, y int) color.Color {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if c := tm.emu.CellAt(x, y); c != nil {
		return c.Style.Fg
	}
	return nil
}

func (tm *term) lines() []string {
	return strings.Split(strings.TrimSuffix(tm.screen(), "\n"), "\n")
}

// column returns the screen column where substr starts in line, or -1.
func column(line, substr string) int {
	i := strings.Index(line, substr)
	if i < 0 {
		return -1
	}
	return len([]rune(line[:i]))
}

func (tm *term) waitUntil(desc string, ok func(screen string) bool) {
	tm.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		s := tm.screen()
		if ok(s) {
			return
		}
		if time.Now().After(deadline) {
			tm.t.Fatalf("timed out waiting for %s. Screen:\n%s", desc, s)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (tm *term) waitFor(text string) {
	tm.t.Helper()
	tm.waitUntil(fmt.Sprintf("%q", text), func(s string) bool { return strings.Contains(s, text) })
}

// waitPrompts waits until n shells have drawn their prompt, so typing does
// not race the shell starting up.
func (tm *term) waitPrompts(n int) {
	tm.t.Helper()
	tm.waitUntil(fmt.Sprintf("%d prompts", n), func(s string) bool { return strings.Count(s, "$ ") >= n })
}

func (tm *term) waitExit() {
	tm.t.Helper()
	select {
	case <-tm.done:
	case <-time.After(10 * time.Second):
		tm.t.Fatalf("floe did not exit. Screen:\n%s", tm.screen())
	}
}

func TestStartsWithOneFramedShell(t *testing.T) {
	tm := launch(t, 80, 24)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine("echo hello-$((40+2))")
	tm.waitFor("hello-42")

	lines := tm.lines()
	t.Logf("screen:\n%s", tm.screen())
	if !strings.HasPrefix(lines[0], "╭─ 1 sh ") || !strings.HasSuffix(lines[0], "╮") {
		t.Errorf("top border looks wrong: %q", lines[0])
	}
	if !strings.HasPrefix(lines[22], "╰") || !strings.HasSuffix(lines[22], "╯") {
		t.Errorf("bottom border should sit just above the status bar: %q", lines[22])
	}
	if !strings.HasPrefix(lines[23], " 1 ") || !strings.Contains(lines[23], "ctrl+b prefix") {
		t.Errorf("status bar looks wrong: %q", lines[23])
	}
}

func TestSplitFocusAndClose(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)

	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)
	tm.typeLine("echo right-pane")
	tm.waitFor("right-pane")

	tm.prefix("-")
	tm.waitFor(" 3 sh ")
	tm.waitPrompts(3)
	t.Logf("after splits:\n%s", tm.screen())

	lines := tm.lines()
	if !strings.HasPrefix(lines[0], "╭─ 1 sh ") || !strings.Contains(lines[0], "╮╭─ 2 sh ") {
		t.Errorf("expected windows 1 and 2 side by side: %q", lines[0])
	}
	// 29 rows of windows split 15/14.
	if !strings.Contains(lines[15], "╭─ 3 sh ") {
		t.Errorf("expected window 3 below window 2: %q", lines[14])
	}

	// Focus left, type there, and check it landed in the left window.
	tm.prefix("h")
	tm.typeLine("echo left-pane")
	tm.waitFor("left-pane")
	for _, l := range tm.lines() {
		if i := column(l, "left-pane"); i > 50 {
			t.Fatalf("typed text appeared in the wrong window: %q", l)
		}
	}

	// Close window 1; window 2 and 3 take the full width.
	tm.prefix("x")
	tm.waitUntil("window 1 to close", func(s string) bool { return !strings.Contains(s, " 1 sh ") })
	if l := tm.lines()[0]; !strings.HasPrefix(l, "╭─ 2 sh ") {
		t.Errorf("expected window 2 at the top left: %q", l)
	}
}

func TestKeyboardResizeGrowsFocusedWindow(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.prefix("h")

	// Window 1 is focused and on top; its top-right corner marks its width.
	before := column(tm.lines()[0], "╮")
	tm.prefix("L")
	tm.send("LL") // repeat without the prefix
	tm.waitUntil("window 1 to grow by 6", func(string) bool {
		return column(tm.lines()[0], "╮") == before+6
	})
	tm.waitFor("REPEAT")
	tm.send("\x1b")
	tm.waitFor("ctrl+b prefix")
}

func TestShellExitClosesWindowAndLastExitQuits(t *testing.T) {
	tm := launch(t, 80, 24)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)

	tm.typeLine("exit")
	tm.waitUntil("window 2 to close", func(s string) bool { return !strings.Contains(s, " 2 sh ") })
	tm.typeLine("exit")
	tm.waitExit()
}

func TestQuitKillsEverything(t *testing.T) {
	tm := launch(t, 80, 24)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)
	tm.prefix("q")
	tm.waitExit()
}

func TestClickFocusesWindow(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)

	// Window 2 is focused: its frame uses the accent color, window 1's not.
	accent, muted := tm.fg(50, 0), tm.fg(0, 0)
	if accent == nil || muted == nil || accent == muted {
		t.Fatalf("focused and unfocused frames should differ: %v vs %v", accent, muted)
	}

	tm.click(10, 10)
	tm.waitUntil("window 1 frame to take the accent", func(string) bool { return tm.fg(0, 0) == accent })
	if tm.fg(50, 0) != muted {
		t.Fatal("window 2 frame should lose the accent")
	}
	tm.typeLine("echo clicked-left")
	tm.waitFor("clicked-left")
	for _, l := range tm.lines() {
		if i := column(l, "clicked-left"); i > 50 {
			t.Fatalf("click did not move focus: %q", l)
		}
	}
}

func TestFullScreenAppRendersInsideWindow(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)
	tm.typeLine("vi -u NONE")
	tm.waitFor("~")
	t.Logf("vi:\n%s", tm.screen())
	tm.send("ihello from vi\x1b")
	tm.waitFor("hello from vi")
	tm.send(":q!\r")
	tm.waitUntil("vi to exit", func(s string) bool { return !strings.Contains(s, "hello from vi") })
}
