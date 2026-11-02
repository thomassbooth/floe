package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// linkAt returns the hyperlink the outer terminal holds for the cell at x, y.
func (tm *term) linkAt(x, y int) (url, params string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if c := tm.emu.CellAt(x, y); c != nil {
		return c.Link.URL, c.Link.Params
	}
	return "", ""
}

// printLink has the focused window's shell print label-2 as a link. The
// label is worked out by the shell, so the command line as typed cannot match
// it.
func (tm *term) printLink(url, params string) {
	tm.t.Helper()
	tm.typeLine(fmt.Sprintf(`printf '\033]8;%s;%s\033\\label-%%s\033]8;;\033\\ after\n' $((1+1))`, params, url))
	tm.waitFor("label-2 after")
}

func TestLinkReachesTheOuterTerminal(t *testing.T) {
	tm := launch(t, 120, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/a?b=c", "")

	x, y := tm.find("label-2 after")
	for i := range len("label-2") {
		if url, _ := tm.linkAt(x+i, y); url != "https://example.com/a?b=c" {
			t.Fatalf("cell %d of the label has link %q", i, url)
		}
	}
	// The link ends with its label.
	if url, _ := tm.linkAt(x+len("label-2"), y); url != "" {
		t.Fatalf("the cell after the label has link %q", url)
	}
}

func TestLinkKeepsItsID(t *testing.T) {
	tm := launch(t, 120, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/", "id=abc")

	x, y := tm.find("label-2 after")
	if _, params := tm.linkAt(x, y); params != "id=abc" {
		t.Fatalf("params = %q, want id=abc", params)
	}
}

func TestLinkWithAnUnsafeSchemeIsDropped(t *testing.T) {
	tm := launch(t, 120, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("javascript:alert(1)", "")

	// The text stays; only the link goes.
	x, y := tm.find("label-2 after")
	if url, _ := tm.linkAt(x, y); url != "" {
		t.Fatalf("the link should be dropped, got %q", url)
	}
}

func TestLinkIsClippedByTheWindowOverIt(t *testing.T) {
	tm := launch(t, 120, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)

	// A row of link cells most of the width of the screen, in the middle of
	// it, so it runs under the small window opened next.
	tm.typeLine(`clear; i=0; while [ $i -lt 14 ]; do echo; i=$((i+1)); done; i=0; printf '\033]8;;https://example.com/\033\\'; while [ $i -lt 110 ]; do printf y; i=$((i+1)); done; printf '\033]8;;\033\\\n'`)
	tm.waitFor(strings.Repeat("y", 110))
	_, y := tm.find(strings.Repeat("y", 110))

	tm.prefix("c")
	tm.waitFor("╭─ 2 sh ")
	wx, wy := tm.find("╭─ 2 sh ")
	if y <= wy {
		t.Fatalf("the row should run under window 2:\n%s", tm.screen())
	}
	tm.waitUntil("the link to stop at window 2", func(string) bool {
		url, _ := tm.linkAt(wx+3, y)
		return url == ""
	})
	if url, _ := tm.linkAt(wx-1, y); url != "https://example.com/" {
		t.Fatalf("the link should still show beside window 2, got %q:\n%s", url, tm.screen())
	}
	if url, _ := tm.linkAt(wx, y); url != "" {
		t.Fatalf("window 2's border should not be a link, got %q", url)
	}
}

func TestLinkSurvivesScrollingBack(t *testing.T) {
	tm := launch(t, 120, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/old", "")
	tm.typeLine(`i=0; while [ $i -lt 60 ]; do echo line-$i; i=$((i+1)); done; echo finished-$((1+1))`)
	tm.waitFor("finished-2")
	tm.waitUntil("the link to scroll away", func(s string) bool { return !strings.Contains(s, "label-2 after") })

	for range 60 {
		tm.send("\x1b[<64;10;10M")
	}
	tm.waitFor("label-2 after")
	x, y := tm.find("label-2 after")
	if url, _ := tm.linkAt(x, y); url != "https://example.com/old" {
		t.Fatalf("the link in the history has link %q", url)
	}
}

// fakeOpener puts a program on the PATH under the name floe opens links with.
// It adds what it is asked to open to the file it returns, so a link opened
// twice shows up too.
func fakeOpener(t *testing.T) (env []string, opened string) {
	t.Helper()
	dir := t.TempDir()
	opened = filepath.Join(dir, "opened")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s' \"$1\" >> %s\n", opened)
	for _, name := range []string{"open", "xdg-open"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return []string{"PATH=" + dir + ":" + os.Getenv("PATH")}, opened
}

// ctrlClick presses and lets go of the left button with ctrl held.
func (tm *term) ctrlClick(x, y int) {
	tm.t.Helper()
	tm.send(fmt.Sprintf("\x1b[<16;%d;%dM\x1b[<16;%d;%dm", x+1, y+1, x+1, y+1))
}

func TestCtrlClickOpensALink(t *testing.T) {
	env, opened := fakeOpener(t)
	tm := launchWith(t, options{w: 120, h: 30, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/open", "")

	x, y := tm.find("label-2 after")
	tm.ctrlClick(x+1, y)
	tm.waitUntil("the link to open", func(string) bool {
		got, err := os.ReadFile(opened)
		return err == nil && string(got) == "https://example.com/open"
	})
	tm.waitFor("opened https://example.com/open")
}

// waitOpened waits for the fake opener to have been asked for exactly url.
func (tm *term) waitOpened(opened, url string) {
	tm.t.Helper()
	tm.waitUntil("the link to open", func(string) bool {
		got, err := os.ReadFile(opened)
		return err == nil && string(got) == url
	})
}

func TestClickOpensALink(t *testing.T) {
	env, opened := fakeOpener(t)
	tm := launchWith(t, options{w: 120, h: 30, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/open", "")

	x, y := tm.find("label-2 after")
	tm.click(x+1, y)
	tm.waitOpened(opened, "https://example.com/open")
	tm.waitFor("opened https://example.com/open")
}

func TestClickCopiesAFileLinkRatherThanRunningIt(t *testing.T) {
	env, opened := fakeOpener(t)
	tm := launchWith(t, options{w: 120, h: 30, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("file:///tmp/evil.command", "")

	x, y := tm.find("label-2 after")
	tm.click(x, y)
	tm.waitCopied("file:///tmp/evil.command")
	tm.waitFor("copied link")
	if _, err := os.Stat(opened); err == nil {
		t.Fatal("a file link must not be opened")
	}
}

func TestDraggingOverALinkSelectsItWithoutOpeningIt(t *testing.T) {
	env, opened := fakeOpener(t)
	tm := launchWith(t, options{w: 120, h: 30, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/", "")

	x, y := tm.find("label-2 after")
	tm.drag(x, y, x+len("label-2")-1, y)
	tm.waitCopied("label-2")
	if _, err := os.Stat(opened); err == nil {
		t.Fatal("a drag must not open the link")
	}
}

func TestDraggingAwayFromALinkAndBackDoesNotOpenIt(t *testing.T) {
	env, opened := fakeOpener(t)
	tm := launchWith(t, options{w: 120, h: 30, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/", "")

	x, y := tm.find("label-2 after")
	tm.press(x, y)
	tm.moveTo(x+3, y)
	tm.moveTo(x, y)
	tm.release(x, y)
	tm.typeLine("echo still-$((1+1))")
	tm.waitFor("still-2")
	if _, err := os.Stat(opened); err == nil {
		t.Fatal("changing your mind mid-drag must not open the link")
	}
}

func TestDoubleClickOnALinkOpensItOnceAndSelectsTheWord(t *testing.T) {
	env, opened := fakeOpener(t)
	tm := launchWith(t, options{w: 120, h: 30, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/once", "")

	x, y := tm.find("label-2 after")
	tm.click(x, y)
	tm.click(x, y)
	tm.waitCopied("label-2")
	tm.waitOpened(opened, "https://example.com/once")
}

func TestClickOnAWindowWithoutFocusOnlyFocusesIt(t *testing.T) {
	env, opened := fakeOpener(t)
	tm := launchWith(t, options{w: 120, h: 30, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/behind", "")
	x, y := tm.find("label-2 after")

	tm.prefix("c")
	tm.waitFor("╭─ 2 sh ")
	tm.waitPrompts(2)
	tm.click(x, y)

	// What is typed next lands in window 1, under the link.
	tm.typeLine("echo focused-$((1+1))")
	tm.waitFor("focused-2")
	if fx, fy := tm.find("focused-2"); fx != x || fy <= y {
		t.Fatalf("window 1 should have focus, output at %d,%d:\n%s", fx, fy, tm.screen())
	}
	if _, err := os.Stat(opened); err == nil {
		t.Fatal("the click that focuses a window must not open a link")
	}

	tm.click(x+1, y)
	tm.waitOpened(opened, "https://example.com/behind")
}

func TestMouseProgramsKeepPlainClicksButNotCtrlClicks(t *testing.T) {
	env, opened := fakeOpener(t)
	tm := launchWith(t, options{w: 80, h: 24, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine(`clear; printf '\033]8;;https://example.com/app\033\\label-%s\033]8;;\033\\\n' $((1+1)); printf '\033[?1002h\033[?1006h'; stty raw -echo; cat -v`)
	tm.waitFor("╭─ 1 cat ")

	x, y := tm.find("label-2")
	tm.click(x, y)
	tm.waitOutput("^[[<0;")
	if _, err := os.Stat(opened); err == nil {
		t.Fatal("a plain click belongs to the program")
	}

	tm.ctrlClick(x, y)
	tm.waitOpened(opened, "https://example.com/app")
}

func TestCtrlClickCopiesAFileLinkRatherThanRunningIt(t *testing.T) {
	env, opened := fakeOpener(t)
	tm := launchWith(t, options{w: 120, h: 30, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("file:///tmp/evil.command", "")

	x, y := tm.find("label-2 after")
	tm.ctrlClick(x, y)
	tm.waitCopied("file:///tmp/evil.command")
	tm.waitFor("copied link")
	if _, err := os.Stat(opened); err == nil {
		t.Fatal("a file link must not be opened")
	}
}

func TestCtrlClickOverSSHCopiesTheLink(t *testing.T) {
	env, opened := fakeOpener(t)
	env = append(env, "SSH_CONNECTION=1.2.3.4 1 5.6.7.8 22")
	tm := launchWith(t, options{w: 120, h: 30, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/far", "")

	// Opening it here would start a browser on the wrong machine.
	x, y := tm.find("label-2 after")
	tm.ctrlClick(x, y)
	tm.waitCopied("https://example.com/far")
	tm.waitFor("copied link")
	if _, err := os.Stat(opened); err == nil {
		t.Fatal("the link must not open on the remote machine")
	}
}

func TestCtrlClickWhereThereIsNoLinkOpensNothing(t *testing.T) {
	env, opened := fakeOpener(t)
	tm := launchWith(t, options{w: 120, h: 30, env: env})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.printLink("https://example.com/", "")

	x, y := tm.find("label-2 after")
	tm.ctrlClick(x+len("label-2 "), y)
	tm.typeLine("echo still-$((1+1))")
	tm.waitFor("still-2")
	if _, err := os.Stat(opened); err == nil {
		t.Fatal("nothing should open")
	}
}
