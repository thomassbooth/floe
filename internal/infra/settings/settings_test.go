package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingFileStartsEmptyAndSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Theme(); ok {
		t.Fatal("no theme should be set yet")
	}
	if err := f.SetTheme("nord"); err != nil {
		t.Fatal(err)
	}

	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := again.Theme(); !ok || got != "nord" {
		t.Fatalf("theme = %q, %v", got, ok)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}

func TestUnknownFieldsAndOlderFilesLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"theme":"dracula","future":true}`), 0o644)
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := f.Theme(); got != "dracula" {
		t.Fatalf("theme = %q", got)
	}
}

func TestCorruptFileIsAnErrorAndLeftAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte("{not json"), 0o644)
	if _, err := Open(path); err == nil {
		t.Fatal("expected an error")
	}
	if raw, _ := os.ReadFile(path); string(raw) != "{not json" {
		t.Fatal("a corrupt file must not be overwritten")
	}
}

func TestDefaultPathHonorsXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/cfg")
	if got, _ := DefaultPath(); got != "/tmp/cfg/floe/settings.json" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	home, _ := os.UserHomeDir()
	if got, _ := DefaultPath(); got != filepath.Join(home, ".config", "floe", "settings.json") {
		t.Fatalf("got %q", got)
	}
}
