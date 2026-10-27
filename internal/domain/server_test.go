package domain

import (
	"errors"
	"testing"
)

func mustCreate(t *testing.T, s *Server, name string) *Session {
	t.Helper()
	sess, err := s.CreateSession(name)
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func TestServerNamesSessions(t *testing.T) {
	s := NewServer()
	if got := mustCreate(t, s, "").Name(); got != "1" {
		t.Fatalf("first auto name = %q", got)
	}
	work := mustCreate(t, s, "  work ")
	if work.Name() != "work" {
		t.Fatalf("names should be trimmed, got %q", work.Name())
	}
	if got := mustCreate(t, s, "").Name(); got != "2" {
		t.Fatalf("second auto name = %q", got)
	}

	if _, err := s.CreateSession("work"); !errors.Is(err, ErrSessionExists) {
		t.Errorf("duplicate name: got %v", err)
	}
	for _, bad := range []string{"   ", "tab\there", "this-name-is-far-too-long-for-the-status-bar"} {
		if _, err := s.CreateSession(bad); !errors.Is(err, ErrInvalidName) {
			t.Errorf("name %q: got %v", bad, err)
		}
	}

	if err := s.RenameSession(work.ID(), "work"); err != nil {
		t.Errorf("renaming to the same name should succeed: %v", err)
	}
	if err := s.RenameSession(work.ID(), "1"); !errors.Is(err, ErrSessionExists) {
		t.Errorf("rename onto a taken name: got %v", err)
	}
	if err := s.RenameSession(work.ID(), "play"); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.SessionByName("play"); !ok || got != work {
		t.Fatal("lookup by new name failed")
	}
}

func TestServerReusesFreedNumbers(t *testing.T) {
	s := NewServer()
	one := mustCreate(t, s, "")
	mustCreate(t, s, "")
	s.RemoveSession(one.ID())
	if got := mustCreate(t, s, "").Name(); got != "1" {
		t.Fatalf("got %q, want the freed name 1", got)
	}
}

func TestServerPaneIDsAreUniqueAcrossSessions(t *testing.T) {
	s := NewServer()
	a := mustCreate(t, s, "a")
	b := mustCreate(t, s, "b")
	a.SetArea(screen)
	b.SetArea(screen)
	pa, pb := s.NewPaneID(), s.NewPaneID()
	if pa == pb {
		t.Fatal("pane IDs collide")
	}
	a.OpenWindow("sh", Right, pa)
	b.OpenWindow("sh", Right, pb)

	if got, ok := s.SessionForPane(pb); !ok || got != b {
		t.Fatal("pane lookup across sessions failed")
	}
}

func TestServerAttachAndRecency(t *testing.T) {
	s := NewServer()
	a := mustCreate(t, s, "a")
	b := mustCreate(t, s, "b")
	s.AddClient(1)

	if got, _ := s.MostRecentSession(0); got != b {
		t.Fatal("newest session should be most recent")
	}
	if err := s.Attach(1, a.ID()); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.MostRecentSession(0); got != a {
		t.Fatal("attaching should make a session most recent")
	}
	if got, _ := s.MostRecentSession(a.ID()); got != b {
		t.Fatal("exclude should skip a")
	}
	if c, _ := s.Client(1); c.Session != a.ID() {
		t.Fatal("client should be attached to a")
	}

	if err := s.Attach(1, 99); !errors.Is(err, ErrUnknownSession) {
		t.Errorf("attach to unknown session: got %v", err)
	}
	if err := s.Attach(2, a.ID()); !errors.Is(err, ErrUnknownClient) {
		t.Errorf("attach unknown client: got %v", err)
	}

	s.RemoveSession(a.ID())
	if c, _ := s.Client(1); c.Session != 0 {
		t.Fatal("removing a session should detach its clients")
	}
}

func TestServerAreaFollowsMostRecentlyActiveClient(t *testing.T) {
	s := NewServer()
	sess := mustCreate(t, s, "a")
	if _, ok := s.AreaFor(sess.ID()); ok {
		t.Fatal("no clients, no area")
	}

	s.AddClient(1)
	s.AddClient(2)
	s.Attach(1, sess.ID())
	s.Attach(2, sess.ID())
	s.SetClientSize(1, Size{W: 80, H: 24})
	if got, _ := s.AreaFor(sess.ID()); got != (Size{W: 80, H: 24}) {
		t.Fatalf("only client 1 has a size, got %+v", got)
	}

	s.SetClientSize(2, Size{W: 200, H: 50})
	if got, _ := s.AreaFor(sess.ID()); got != (Size{W: 200, H: 50}) {
		t.Fatalf("client 2 attached last, got %+v", got)
	}

	s.Touch(1)
	if got, _ := s.AreaFor(sess.ID()); got != (Size{W: 80, H: 24}) {
		t.Fatalf("client 1 is now most active, got %+v", got)
	}

	if ids := s.ClientsOf(sess.ID()); len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("clients of session = %v", ids)
	}
	if infos := s.Infos(); len(infos) != 1 || infos[0].Clients != 2 {
		t.Fatalf("infos = %+v", infos)
	}

	s.RemoveClient(1)
	if got, _ := s.AreaFor(sess.ID()); got != (Size{W: 200, H: 50}) {
		t.Fatalf("after client 1 leaves, got %+v", got)
	}
}

func TestServerSaveAndRestoreRoundTrip(t *testing.T) {
	s := NewServer()
	work := mustCreate(t, s, "work")
	work.SetArea(screen)
	work.OpenWindow("vim", Right, s.NewPaneID())
	work.OpenWindow("sh", Right, s.NewPaneID())
	work.Snap(2, ZoneRight)
	work.Focus(1)
	other := mustCreate(t, s, "other")
	other.SetArea(Rect{W: 100, H: 30})
	other.OpenWindow("sh", Right, s.NewPaneID())

	saved := s.Save()
	if len(saved) != 2 || saved[0].Name != "work" || len(saved[0].Windows) != 2 {
		t.Fatalf("saved = %+v", saved)
	}

	fresh := NewServer()
	for _, ss := range saved {
		if _, err := fresh.RestoreSession(ss, "sh"); err != nil {
			t.Fatal(err)
		}
	}
	got, ok := fresh.SessionByName("work")
	if !ok {
		t.Fatal("work was not restored")
	}
	before, after := work.Snapshot(), got.Snapshot()
	if before.Area != after.Area || before.Focused != after.Focused || len(before.Windows) != len(after.Windows) {
		t.Fatalf("before %+v\nafter  %+v", before, after)
	}
	for i := range before.Windows {
		b, a := before.Windows[i], after.Windows[i]
		if b.ID != a.ID || b.Frame != a.Frame || b.Restore != a.Restore || b.Focused != a.Focused {
			t.Errorf("window %d: before %+v, after %+v", i, b, a)
		}
	}
	// Programs are not restored, so neither are their titles.
	if after.Windows[1].Title != "sh" {
		t.Fatalf("restored title = %q, want the shell's", after.Windows[1].Title)
	}
	// Restored windows get new panes, never ones shown elsewhere.
	if a, b := after.Windows[0].Pane, after.Windows[1].Pane; a == b || a == 0 {
		t.Fatalf("panes %d and %d", a, b)
	}
	if _, err := fresh.RestoreSession(saved[0], "sh"); err == nil {
		t.Fatal("a name already in use cannot be restored twice")
	}
	// New windows continue the numbering.
	w, _ := got.OpenWindow("sh", Down, fresh.NewPaneID())
	if w.ID != 3 {
		t.Fatalf("next window = %d, want 3", w.ID)
	}

	bad := SavedSession{Name: "bad", Windows: []SavedWindow{{ID: 1}, {ID: 1}}}
	if _, err := fresh.RestoreSession(bad, "sh"); err == nil {
		t.Fatal("duplicate window numbers should be refused")
	}
	if _, ok := fresh.SessionByName("bad"); ok {
		t.Fatal("a refused session must not be left behind")
	}
}
