package input

import (
	"slices"
	"strings"
	"testing"

	"github.com/thomassbooth/floe/internal/domain"
)

func mustParse(t *testing.T, src string) Keymap {
	t.Helper()
	km, errs := ParseConfig(src, DefaultKeymap())
	if len(errs) > 0 {
		t.Fatalf("ParseConfig: %v", errs)
	}
	return km
}

// keysOf returns the keys of the named binding in table, as text.
func keysOf(t *testing.T, table []Binding, name string) string {
	t.Helper()
	i := slices.IndexFunc(table, func(b Binding) bool { return b.Name == name })
	if i < 0 {
		t.Fatalf("no binding %q", name)
	}
	var names []string
	for _, k := range table[i].Keys {
		names = append(names, k.String())
	}
	return strings.Join(names, " ")
}

func TestEmptyConfigKeepsTheDefaults(t *testing.T) {
	km := mustParse(t, "\n  # nothing here\n\n")
	if km.Prefix != DefaultKeymap().Prefix || keysOf(t, km.Bindings, "split-right") != "| %" {
		t.Fatalf("defaults changed: %+v", km)
	}
}

func TestBindAddsAKeyAndTakesItFromOthers(t *testing.T) {
	km := mustParse(t, "bind s split-down\nbind ctrl+left focus-left\n")
	if got := keysOf(t, km.Bindings, "split-down"); got != "s - \"" {
		t.Errorf("split-down = %q", got)
	}
	if got := keysOf(t, km.Bindings, "sessions"); got != "" {
		t.Errorf("sessions should lose s: %q", got)
	}
	if got := keysOf(t, km.Bindings, "focus-left"); got != "ctrl+left h left" {
		t.Errorf("focus-left = %q", got)
	}
}

func TestLaterLinesWin(t *testing.T) {
	km := mustParse(t, "bind v split-right\nbind v split-down\n")
	if keysOf(t, km.Bindings, "split-right") != "| %" || keysOf(t, km.Bindings, "split-down") != "v - \"" {
		t.Fatalf("v should end up on split-down: %v", km.Bindings)
	}
}

func TestUnbindAndUnbindAll(t *testing.T) {
	km := mustParse(t, "unbind %\nunbind --arrange all\nbind --arrange w move-up\n")
	if got := keysOf(t, km.Bindings, "split-right"); got != "|" {
		t.Errorf("split-right = %q", got)
	}
	for _, b := range km.Arrange {
		if b.Name != "move-up" && len(b.Keys) > 0 {
			t.Errorf("%s should have no keys: %v", b.Name, b.Keys)
		}
	}
	if got := keysOf(t, km.Arrange, "move-up"); got != "w" {
		t.Errorf("move-up = %q", got)
	}
}

func TestPrefixReplacesTheDefaultAndShadowsBindings(t *testing.T) {
	km := mustParse(t, "prefix a\n")
	if km.Prefix != (domain.Key{Code: 'a', Text: "a"}) {
		t.Fatalf("prefix = %#v", km.Prefix)
	}
	// a was arrange's key; after the prefix it now sends a itself.
	if got := keysOf(t, km.Bindings, "arrange"); got != "" {
		t.Errorf("arrange should lose a: %q", got)
	}
	m := NewMachine(km)
	m.Handle(char('a'))
	if res, ok := m.Handle(char('a')); !ok || res.Command != (domain.SendKey{Key: char('a')}) {
		t.Errorf("prefix twice should send it: %#v", res)
	}
}

func TestCustomKeymapDrivesTheMachine(t *testing.T) {
	m := NewMachine(mustParse(t, "prefix ctrl+a\nbind v split-right\nbind --arrange w done\n"))
	ctrlA := domain.Key{Code: 'a', Mod: domain.ModCtrl}
	if res, _ := m.Handle(prefix); res.Command != (domain.SendKey{Key: prefix}) {
		t.Fatalf("ctrl+b should go to the pane: %#v", res)
	}
	m.Handle(ctrlA)
	if res, _ := m.Handle(char('v')); res.Command != (domain.SplitWindow{Dir: domain.Right}) {
		t.Fatalf("v should split right: %#v", res)
	}
	m.Handle(ctrlA)
	m.Handle(char('a'))
	if m.Mode() != Arrange {
		t.Fatal("a should still enter arrange mode")
	}
	m.Handle(char('w'))
	if m.Mode() != Normal {
		t.Fatal("w should leave arrange mode")
	}
}

func TestBrokenConfigReportsEveryLineAndChangesNothing(t *testing.T) {
	src := strings.Join([]string{
		"prefix ctrl+a",           // fine, but not applied
		"bind v splt-right",       // 2
		"bind q",                  // 3
		"bnd v split-right",       // 4
		"bind hyper+x tidy",       // 5
		"bind done done",          // 6: not a key
		"bind w done",             // 7: arrange only
		"bind --arrange d detach", // 8: not in arrange mode
		"bind ctrl+a tidy",        // 9: the prefix
		"bind esc tidy",           // 10
		"unbind",                  // 11
	}, "\n")
	km, errs := ParseConfig(src, DefaultKeymap())
	if km.Prefix != DefaultKeymap().Prefix {
		t.Error("a broken file should not change the prefix")
	}
	var lines []int
	for _, e := range errs {
		lines = append(lines, e.Line)
	}
	if want := []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11}; !slices.Equal(lines, want) {
		t.Fatalf("errors on lines %v, want %v: %v", lines, want, errs)
	}
	for i, want := range map[int]string{
		0: `line 2: unknown action "splt-right" (floe keys lists them)`,
		5: `unknown action "done": it is only in arrange mode; use bind --arrange`,
		6: `unknown action "detach": it is not in arrange mode`,
		7: "ctrl+a is the prefix",
		8: "esc cancels the prefix",
	} {
		if !strings.Contains(errs[i].Error(), want) {
			t.Errorf("error %d = %q, want it to mention %q", i, errs[i], want)
		}
	}
}

func TestEveryBindingHasAUniqueName(t *testing.T) {
	km := DefaultKeymap()
	for _, table := range [][]Binding{km.Bindings, km.Arrange} {
		seen := map[string]bool{}
		for _, b := range table {
			if b.Name == "" || seen[b.Name] {
				t.Errorf("binding %q: names must be set and unique", b.Name)
			}
			seen[b.Name] = true
		}
	}
}
