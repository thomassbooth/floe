package input

import (
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

var prefix = domain.Key{Code: 'b', Mod: domain.ModCtrl}

type step struct {
	key  domain.Key
	want any // a domain.Command, an Action, or nil for nothing
	mode Mode
}

func run(t *testing.T, steps []step) {
	t.Helper()
	m := NewMachine(DefaultKeymap())
	for i, s := range steps {
		got, ok := m.Handle(s.key)
		var want Result
		switch w := s.want.(type) {
		case domain.Command:
			want.Command = w
		case Action:
			want.Action = w
		}
		if (s.want == nil) == ok {
			t.Fatalf("step %d (%s): got %+v (%v), want %v", i, s.key, got, ok, s.want)
		}
		if ok && got != want {
			t.Fatalf("step %d (%s): got %#v, want %#v", i, s.key, got, want)
		}
		if m.Mode() != s.mode {
			t.Fatalf("step %d (%s): mode %v, want %v", i, s.key, m.Mode(), s.mode)
		}
	}
}

func TestPlainKeysGoToThePane(t *testing.T) {
	a := char('a')
	run(t, []step{
		{a, domain.SendKey{Key: a}, Normal},
		{char('|'), domain.SendKey{Key: char('|')}, Normal},
	})
}

func TestPrefixRunsOneBinding(t *testing.T) {
	run(t, []step{
		{prefix, nil, Prefix},
		{char('|'), domain.SplitWindow{Dir: domain.Right}, Normal},
		{char('|'), domain.SendKey{Key: char('|')}, Normal},
	})
}

func TestPrefixTwiceSendsThePrefix(t *testing.T) {
	run(t, []step{
		{prefix, nil, Prefix},
		{prefix, domain.SendKey{Key: prefix}, Normal},
	})
}

func TestEscapeAndUnknownKeysCancelThePrefix(t *testing.T) {
	run(t, []step{
		{prefix, nil, Prefix},
		{special(domain.KeyEscape, 0), nil, Normal},
		{prefix, nil, Prefix},
		{char('y'), nil, Normal},
	})
}

func TestResizeRepeatsWithoutPrefix(t *testing.T) {
	grow := domain.ResizeWindow{Dir: domain.Right, Amount: resizeStep}
	shrink := domain.ResizeWindow{Dir: domain.Left, Amount: resizeStep}
	run(t, []step{
		{prefix, nil, Prefix},
		{char('L'), grow, Repeat},
		{char('L'), grow, Repeat},
		{special(domain.KeyLeft, domain.ModShift), shrink, Repeat},
		// A non-repeatable key ends repeat mode and is handled normally.
		{char('a'), domain.SendKey{Key: char('a')}, Normal},
	})
}

func TestRepeatModeHandsPrefixBackToNormal(t *testing.T) {
	run(t, []step{
		{prefix, nil, Prefix},
		{char('L'), domain.ResizeWindow{Dir: domain.Right, Amount: resizeStep}, Repeat},
		{prefix, nil, Prefix},
		{char('-'), domain.SplitWindow{Dir: domain.Down}, Normal},
	})
}

func TestNewWindowBinding(t *testing.T) {
	run(t, []step{
		{prefix, nil, Prefix},
		{char('c'), ActionNewWindow, Normal},
	})
}

func TestSessionBindings(t *testing.T) {
	run(t, []step{
		{prefix, nil, Prefix},
		{char('s'), ActionSessionSwitcher, Normal},
		{prefix, nil, Prefix},
		{char('C'), domain.CreateSession{}, Normal},
		{prefix, nil, Prefix},
		{char('$'), ActionRenameSession, Normal},
		{prefix, nil, Prefix},
		{char(')'), ActionNextSession, Normal},
	})
}

func TestArrangeModeCapturesKeysUntilDone(t *testing.T) {
	run(t, []step{
		{prefix, nil, Prefix},
		{char('a'), ActionArrange, Arrange},
		{char('l'), domain.MoveWindow{Dir: domain.Right, Amount: 2}, Arrange},
		{char('j'), domain.MoveWindow{Dir: domain.Down, Amount: 1}, Arrange},
		{char('L'), domain.ResizeWindow{Dir: domain.Right, Amount: 2}, Arrange},
		{special(domain.KeyTab, domain.ModShift), domain.CycleFocus{Delta: -1}, Arrange},
		// Keys without a binding are swallowed, not typed into the pane.
		{char('w'), nil, Arrange},
		{prefix, nil, Arrange},
		{special(domain.KeyEscape, 0), ActionArrange, Normal},
		{char('l'), domain.SendKey{Key: char('l')}, Normal},
	})
}

func TestToggleArrangeAndReset(t *testing.T) {
	m := NewMachine(DefaultKeymap())
	m.ToggleArrange()
	if m.Mode() != Arrange {
		t.Fatal("expected arrange mode")
	}
	m.Reset()
	if m.Mode() != Arrange {
		t.Fatal("reset must not leave arrange mode")
	}
	m.ToggleArrange()
	if m.Mode() != Normal {
		t.Fatal("expected normal mode")
	}
	m.Handle(prefix)
	m.Reset()
	if m.Mode() != Normal {
		t.Fatal("reset should cancel the prefix")
	}
}
