package store

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/thomassbooth/floe/internal/domain"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	f := New(filepath.Join(t.TempDir(), "floe", "sessions.json"))
	if got, err := f.Load(); err != nil || got != nil {
		t.Fatalf("a missing file is no sessions: %v, %v", got, err)
	}
	saved := []domain.SavedSession{
		{
			Name: "work",
			Area: domain.Rect{W: 80, H: 23},
			Windows: []domain.SavedWindow{
				{ID: 2, Frame: domain.Rect{X: 40, W: 40, H: 23}, Minimized: true, Dir: "/tmp"},
				{ID: 1, Frame: domain.Rect{W: 40, H: 23}, Restore: domain.Rect{X: 5, Y: 5, W: 20, H: 8}, Focused: true},
			},
		},
		{Name: "empty", Area: domain.Rect{W: 10, H: 10}},
	}
	// Pane means nothing after a restart, so it is not written down.
	requireEveryFieldSet(t, saved[0].Windows, "Pane")
	if err := f.Save(saved); err != nil {
		t.Fatal(err)
	}
	got, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[0], saved[0]) || got[1].Name != "empty" || len(got[1].Windows) != 0 {
		t.Fatalf("got %+v\nwant %+v", got, saved)
	}
}

func TestLoadRejectsUnknownVersionsAndJunk(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"future.json": `{"version": 99, "sessions": []}`,
		"junk.json":   `{not json`,
	} {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte(body), 0o600)
		if _, err := New(path).Load(); err == nil {
			t.Errorf("%s should not load", name)
		}
	}
}

func TestDefaultPathHonorsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/state")
	if got, _ := DefaultPath(); got != "/tmp/state/floe/sessions.json" {
		t.Fatalf("got %q", got)
	}
}

// requireEveryFieldSet fails unless each field of T, apart from skip, has
// a value in at least one of items, so a field added later cannot pass the
// round trip just by being zero on both sides.
func requireEveryFieldSet[T any](t *testing.T, items []T, skip ...string) {
	t.Helper()
	typ := reflect.TypeFor[T]()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		if slices.Contains(skip, name) {
			continue
		}
		set := false
		for _, item := range items {
			set = set || !reflect.ValueOf(item).Field(i).IsZero()
		}
		if !set {
			t.Errorf("no %s in the test sets %s; give one a value so the round trip covers it", typ.Name(), name)
		}
	}
}
