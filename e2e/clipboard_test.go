package e2e

import "testing"

// copied returns what floe last put on the outer terminal's clipboard.
func (tm *term) copied() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.clipboard
}

func (tm *term) waitCopied(want string) {
	tm.t.Helper()
	tm.waitUntil("the clipboard to hold "+want, func(string) bool { return tm.copied() == want })
}

func TestProgramsCanCopyToTheClipboard(t *testing.T) {
	tm := launch(t, 80, 24)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	// What nvim and friends send when you yank: OSC 52 with base64 text.
	tm.typeLine(`printf '\033]52;c;%s\007' "$(printf 'yanked text' | base64)"`)
	tm.waitCopied("yanked text")
}

func TestDragSelectsAndCopiesText(t *testing.T) {
	tm := launch(t, 80, 24)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine("clear; printf 'alpha beta gamma\\ndelta epsilon\\n'")
	tm.waitFor("delta epsilon")
	// After clear: row 1 is "alpha beta gamma", row 2 "delta epsilon";
	// the window's content starts at column 1.
	tm.drag(7, 1, 5, 2) // from "beta" to the end of "delta"
	tm.waitCopied("beta gamma\ndelta")
	tm.waitFor("✓ copied 16 characters")
	if !sameColor(tm.bgAt(7, 1), violet) {
		t.Fatalf("the selection should be highlighted, got %v", tm.bgAt(7, 1))
	}

	tm.send("x") // typing clears the highlight
	tm.waitUntil("the highlight to clear", func(string) bool { return !sameColor(tm.bgAt(7, 1), violet) })
}

func TestDoubleAndTripleClickSelectWordAndLine(t *testing.T) {
	tm := launch(t, 80, 24)
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.typeLine("clear; printf 'one ~/path/to/file three\\n'")
	tm.waitFor("one ~/path/to/file three")

	tm.click(10, 1)
	tm.click(10, 1)
	tm.waitCopied("~/path/to/file")

	tm.click(3, 1)
	tm.click(3, 1)
	tm.click(3, 1)
	tm.waitCopied("one ~/path/to/file three")
}
