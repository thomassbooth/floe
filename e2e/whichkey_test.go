package e2e

import (
	"strings"
	"testing"
)

func TestPrefixShowsCommandList(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")

	tm.send("\x02")
	tm.waitFor("Sessions")
	t.Logf("which-key:\n%s", tm.screen())
	screen := tm.screen()
	for _, want := range []string{"Windows", "split right", "hjkl", "focus", "HJKL", "resize", "kill session", "ctrl+b again sends it · esc cancel"} {
		if !strings.Contains(screen, want) {
			t.Errorf("command list is missing %q", want)
		}
	}
	// The groups sit side by side on the same rows.
	for _, l := range tm.lines() {
		if strings.Contains(l, "Windows") && !strings.Contains(l, "Sessions") {
			t.Errorf("groups should share a row: %q", l)
		}
	}
	if bar := tm.lines()[29]; !strings.Contains(bar, "PREFIX") || strings.Contains(bar, "split right") {
		t.Errorf("status bar should only show the PREFIX badge: %q", bar)
	}

	tm.send("\x1b")
	tm.waitUntil("command list to close", func(s string) bool { return !strings.Contains(s, "Sessions") })
}

func TestCommandListClosesAfterACommand(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.send("\x02")
	tm.waitFor("Sessions")
	tm.send("|")
	tm.waitFor(" 2 sh ")
	tm.waitUntil("command list to close", func(s string) bool { return !strings.Contains(s, "Sessions") })
}

func TestCommandListStacksWhenNarrow(t *testing.T) {
	tm := launch(t, 40, 40)
	tm.waitFor(" 1 sh ")
	tm.send("\x02")
	tm.waitFor("Sessions")
	t.Logf("narrow which-key:\n%s", tm.screen())
	for _, l := range tm.lines() {
		if strings.Contains(l, "Windows") && strings.Contains(l, "Sessions") {
			t.Fatalf("groups should stack when narrow: %q", l)
		}
	}
}
