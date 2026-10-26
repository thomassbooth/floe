package e2e

import (
	"strings"
	"testing"
)

// badge returns the session name shown at the left of the status bar.
func (tm *term) badge() string {
	lines := tm.lines()
	bar := lines[len(lines)-1]
	fields := strings.Fields(bar)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func (tm *term) waitBadge(name string) {
	tm.t.Helper()
	tm.waitUntil("session "+name+" in the status bar", func(string) bool { return tm.badge() == name })
}

func TestCreateAndCycleSessions(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine("echo in-first")
	tm.waitFor("in-first")

	tm.prefix("C")
	tm.waitBadge("2")
	tm.waitUntil("a fresh screen", func(s string) bool { return !strings.Contains(s, "in-first") })
	tm.waitPrompts(1)
	tm.typeLine("echo in-second")
	tm.waitFor("in-second")

	tm.prefix("(")
	tm.waitBadge("1")
	tm.waitFor("in-first")
	if strings.Contains(tm.screen(), "in-second") {
		t.Fatal("session 2's output leaked into session 1")
	}

	tm.prefix(")")
	tm.waitBadge("2")
	tm.waitFor("in-second")
}

func TestSessionSwitcher(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.prefix("C")
	tm.waitBadge("2")

	tm.prefix("s")
	tm.waitFor("Sessions")
	t.Logf("switcher:\n%s", tm.screen())
	screen := tm.screen()
	if !strings.Contains(screen, "● 2") || !strings.Contains(screen, "1 window") {
		t.Fatalf("switcher should list both sessions with the current one marked:\n%s", screen)
	}

	tm.send("k")
	tm.send("\r")
	tm.waitBadge("1")
	tm.waitUntil("switcher to close", func(s string) bool { return !strings.Contains(s, "Sessions") })

	// Kill session 2 from the switcher; only session 1 remains.
	tm.prefix("s")
	tm.waitFor("Sessions")
	tm.send("j")
	tm.send("x")
	tm.waitUntil("session 2 to disappear", func(s string) bool {
		return !strings.Contains(s, "  2  ") && !strings.Contains(s, "● 2") && strings.Count(s, "window") == 1
	})
	tm.send("\x1b")
	tm.waitUntil("switcher to close", func(s string) bool { return !strings.Contains(s, "Sessions") })
	tm.waitBadge("1")
}

func TestRenameSession(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")

	tm.prefix("$")
	tm.waitFor("rename session")
	tm.send("\x15") // ctrl+u clears the current name
	tm.send("work")
	tm.waitFor("rename session  work")
	tm.send("\r")
	tm.waitBadge("work")

	// Escape cancels without renaming.
	tm.prefix("$")
	tm.waitFor("rename session")
	tm.send("X\x1b")
	tm.waitUntil("prompt to close", func(s string) bool { return !strings.Contains(s, "rename session") })
	if tm.badge() != "work" {
		t.Fatalf("badge = %q", tm.badge())
	}
}

func TestKillingCurrentSessionMovesToAnother(t *testing.T) {
	tm := launch(t, 100, 30)
	tm.waitFor(" 1 sh ")
	tm.prefix("C")
	tm.waitBadge("2")

	tm.prefix("q")
	tm.waitBadge("1")
	tm.prefix("q")
	tm.waitExit()
}
