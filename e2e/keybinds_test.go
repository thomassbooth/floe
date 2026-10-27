package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeKeybinds puts a keybinds.conf in the config directory config.
func writeKeybinds(t *testing.T, config, text string) {
	t.Helper()
	dir := filepath.Join(config, "floe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keybinds.conf"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCustomPrefixAndBinding(t *testing.T) {
	config := t.TempDir()
	writeKeybinds(t, config, `
# Like screen.
prefix ctrl+a
bind v split-right
unbind |
`)
	tm := launchWith(t, options{w: 100, h: 30, config: config})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	tm.waitFor("ctrl+a prefix")

	// The old prefix goes to the program like any other key.
	tm.send("\x02")
	if strings.Contains(tm.lines()[29], "PREFIX") {
		t.Fatalf("ctrl+b should no longer be the prefix: %q", tm.lines()[29])
	}

	tm.send("\x01")
	tm.waitFor("PREFIX")
	t.Logf("which-key:\n%s", tm.screen())
	if !strings.Contains(tm.screen(), "ctrl+a again sends it") {
		t.Error("the command list should name the new prefix")
	}
	tm.send("v")
	tm.waitFor(" 2 sh ")
	tm.waitPrompts(2)

	// | was unbound, so it does nothing after the prefix.
	tm.send("\x01")
	tm.waitFor("PREFIX")
	tm.send("|")
	tm.waitUntil("the prefix to end", func(s string) bool { return !strings.Contains(s, "PREFIX") })
	if strings.Contains(tm.screen(), " 3 sh ") {
		t.Fatal("| should no longer split")
	}
}

func TestCustomArrangeBinding(t *testing.T) {
	config := t.TempDir()
	writeKeybinds(t, config, "bind --arrange w done\n")
	tm := launchWith(t, options{w: 100, h: 30, config: config})
	tm.waitFor(" 1 sh ")
	tm.prefix("a")
	tm.waitFor("ARRANGE")
	tm.send("w")
	tm.waitFor("ctrl+b prefix")
}

func TestBrokenKeybindsFallBackToDefaults(t *testing.T) {
	config := t.TempDir()
	writeKeybinds(t, config, "prefix ctrl+a\nbind v splt-right\n")
	tm := launchWith(t, options{w: 120, h: 30, config: config})
	tm.waitFor(`keybinds.conf line 2: unknown action "splt-right"`)
	t.Logf("screen:\n%s", tm.screen())

	// None of the file applies, so the defaults still work.
	tm.prefix("|")
	tm.waitFor(" 2 sh ")
	tm.waitFor("ctrl+b prefix")
}

func TestKeysCommandListsBindings(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e tests are slow")
	}
	config := t.TempDir()
	writeKeybinds(t, config, "prefix ctrl+a\nbind v split-right\n")
	cmd := exec.Command(binary, "keys")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+config)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("floe keys: %v: %s", err, out)
	}
	t.Logf("floe keys:\n%s", out)
	for _, want := range []string{"prefix ctrl+a", "split-right", "v |", "focus-next", "--arrange", "done"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("floe keys is missing %q", want)
		}
	}

	writeKeybinds(t, config, "bind v nope\n")
	cmd = exec.Command(binary, "keys")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+config)
	if out, err = cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), `line 1: unknown action "nope"`) {
		t.Errorf("floe keys should report a broken file: %v: %s", err, out)
	}
}
