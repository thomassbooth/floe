// Package store keeps sessions on disk between runs, so closing the
// terminal does not lose them.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/infra/atomicfile"
)

// version is bumped when the file format changes in a way older code
// cannot read.
const version = 1

// File is the sessions file.
type File struct {
	path string
}

// DefaultPath returns where sessions are kept: $XDG_STATE_HOME/floe, or
// ~/.local/state/floe, on every platform.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "floe", "sessions.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find sessions file: %w", err)
	}
	return filepath.Join(home, ".local", "state", "floe", "sessions.json"), nil
}

// New returns the sessions file at path. Nothing is read until Load.
func New(path string) *File {
	return &File{path: path}
}

// The file format. It is kept apart from the domain types so they can
// change without breaking saved files.
type (
	fileData struct {
		Version  int           `json:"version"`
		Sessions []fileSession `json:"sessions"`
	}
	fileSession struct {
		Name    string       `json:"name"`
		Area    fileRect     `json:"area"`
		Windows []fileWindow `json:"windows"`
	}
	fileWindow struct {
		ID      int       `json:"id"`
		Frame   fileRect  `json:"frame"`
		Restore *fileRect `json:"restore,omitempty"`
		Focused bool      `json:"focused,omitempty"`
		Dir     string    `json:"dir,omitempty"`
	}
	fileRect struct {
		X, Y, W, H int
	}
)

// Load reads the saved sessions. A missing file means none were saved.
func (f *File) Load() ([]domain.SavedSession, error) {
	raw, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load sessions: %w", err)
	}
	var d fileData
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("load sessions from %s: %w", f.path, err)
	}
	if d.Version != version {
		return nil, fmt.Errorf("load sessions from %s: unknown version %d", f.path, d.Version)
	}
	out := make([]domain.SavedSession, len(d.Sessions))
	for i, s := range d.Sessions {
		out[i] = domain.SavedSession{Name: s.Name, Area: s.Area.domain()}
		for _, w := range s.Windows {
			sw := domain.SavedWindow{
				ID:      domain.WindowID(w.ID),
				Frame:   w.Frame.domain(),
				Focused: w.Focused,
				Dir:     w.Dir,
			}
			if w.Restore != nil {
				sw.Restore = w.Restore.domain()
			}
			out[i].Windows = append(out[i].Windows, sw)
		}
	}
	return out, nil
}

// Save replaces the saved sessions.
func (f *File) Save(sessions []domain.SavedSession) error {
	d := fileData{Version: version, Sessions: make([]fileSession, len(sessions))}
	for i, s := range sessions {
		d.Sessions[i] = fileSession{Name: s.Name, Area: toFileRect(s.Area), Windows: []fileWindow{}}
		for _, w := range s.Windows {
			fw := fileWindow{
				ID:      int(w.ID),
				Frame:   toFileRect(w.Frame),
				Focused: w.Focused,
				Dir:     w.Dir,
			}
			if !w.Restore.Empty() {
				r := toFileRect(w.Restore)
				fw.Restore = &r
			}
			d.Sessions[i].Windows = append(d.Sessions[i].Windows, fw)
		}
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("save sessions: %w", err)
	}
	if err := atomicfile.Write(f.path, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("save sessions: %w", err)
	}
	return nil
}

func (r fileRect) domain() domain.Rect {
	return domain.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
}

func toFileRect(r domain.Rect) fileRect {
	return fileRect{X: r.X, Y: r.Y, W: r.W, H: r.H}
}
