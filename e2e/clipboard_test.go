package e2e

import "testing"

// copied returns what mux last put on the outer terminal's clipboard.
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
