// Package settings stores the choices people make in the client, such as
// the theme, in a small JSON file so they outlive the session.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// File is a settings file on disk. Its methods are safe to call from any
// goroutine.
type File struct {
	path string
	mu   sync.Mutex
	data data
}

// data is what is saved. New fields must be optional so older files still
// load.
type data struct {
	Theme string `json:"theme,omitempty"`
}

// DefaultPath returns where settings live: $XDG_CONFIG_HOME/mux, or
// ~/.config/mux, on every platform.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "mux", "settings.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find settings: %w", err)
	}
	return filepath.Join(home, ".config", "mux", "settings.json"), nil
}

// Open loads the settings file at path. A missing file is fine: it is
// created on the first save. A file that cannot be read or parsed is an
// error, so it is never overwritten by accident.
func Open(path string) (*File, error) {
	f := &File{path: path}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	if err := json.Unmarshal(raw, &f.data); err != nil {
		return nil, fmt.Errorf("read settings %s: %w", path, err)
	}
	return f, nil
}

// Theme returns the saved theme name, if one was chosen.
func (f *File) Theme() (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data.Theme, f.data.Theme != ""
}

// SetTheme saves the chosen theme name.
func (f *File) SetTheme(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	next := f.data
	next.Theme = name
	if err := f.save(next); err != nil {
		return err
	}
	f.data = next
	return nil
}

// save writes the file atomically: a crash leaves either the old file or
// the new one, never half of one.
func (f *File) save(d data) error {
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".settings-*.json")
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("save settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	if err := os.Rename(tmp.Name(), f.path); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}
