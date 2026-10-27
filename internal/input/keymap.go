package input

import "github.com/thomassbooth/floe/internal/domain"

// resizeStep is how many cells one resize key press moves a divider.
const resizeStep = 2

func char(r rune) domain.Key { return domain.Key{Code: r, Text: string(r)} }

func special(r rune, mod domain.KeyMod) domain.Key { return domain.Key{Code: r, Mod: mod} }

// DefaultKeymap returns tmux-like bindings behind ctrl+b. Bindings without
// keys are there to be given some in keybinds.conf.
func DefaultKeymap() Keymap {
	return Keymap{
		Prefix: domain.Key{Code: 'b', Mod: domain.ModCtrl},
		Bindings: []Binding{
			{Name: "new-window", Keys: []domain.Key{char('c')}, Action: ActionNewWindow, Group: "Windows", Help: "new window"},
			{Name: "split-right", Keys: []domain.Key{char('|'), char('%')}, Command: domain.SplitWindow{Dir: domain.Right}, Group: "Windows", Help: "split right"},
			{Name: "split-down", Keys: []domain.Key{char('-'), char('"')}, Command: domain.SplitWindow{Dir: domain.Down}, Group: "Windows", Help: "split down"},

			{Name: "focus-left", Keys: []domain.Key{char('h'), special(domain.KeyLeft, 0)}, Command: domain.FocusDirection{Dir: domain.Left}, Group: "Windows", Help: "focus left"},
			{Name: "focus-down", Keys: []domain.Key{char('j'), special(domain.KeyDown, 0)}, Command: domain.FocusDirection{Dir: domain.Down}, Group: "Windows", Help: "focus down"},
			{Name: "focus-up", Keys: []domain.Key{char('k'), special(domain.KeyUp, 0)}, Command: domain.FocusDirection{Dir: domain.Up}, Group: "Windows", Help: "focus up"},
			{Name: "focus-right", Keys: []domain.Key{char('l'), special(domain.KeyRight, 0)}, Command: domain.FocusDirection{Dir: domain.Right}, Group: "Windows", Help: "focus right"},

			{Name: "resize-left", Keys: []domain.Key{char('H'), special(domain.KeyLeft, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Left, Amount: resizeStep}, Group: "Windows", Help: "resize left", Repeat: true},
			{Name: "resize-down", Keys: []domain.Key{char('J'), special(domain.KeyDown, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Down, Amount: resizeStep}, Group: "Windows", Help: "resize down", Repeat: true},
			{Name: "resize-up", Keys: []domain.Key{char('K'), special(domain.KeyUp, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Up, Amount: resizeStep}, Group: "Windows", Help: "resize up", Repeat: true},
			{Name: "resize-right", Keys: []domain.Key{char('L'), special(domain.KeyRight, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Right, Amount: resizeStep}, Group: "Windows", Help: "resize right", Repeat: true},

			{Name: "tidy", Keys: []domain.Key{char('=')}, Command: domain.Tidy{}, Group: "Windows", Help: "tidy"},
			{Name: "arrange", Keys: []domain.Key{char('a')}, Action: ActionArrange, Group: "Windows", Help: "arrange"},
			{Name: "maximize", Keys: []domain.Key{char('z')}, Command: domain.ToggleMaximize{}, Group: "Windows", Help: "maximize"},
			{Name: "close", Keys: []domain.Key{char('x')}, Command: domain.CloseWindow{}, Group: "Windows", Help: "close"},
			{Name: "focus-next", Command: domain.CycleFocus{Delta: 1}, Group: "Windows", Help: "next window"},
			{Name: "focus-prev", Command: domain.CycleFocus{Delta: -1}, Group: "Windows", Help: "previous window"},

			{Name: "sessions", Keys: []domain.Key{char('s')}, Action: ActionSessionSwitcher, Group: "Sessions", Help: "sessions"},
			{Name: "new-session", Keys: []domain.Key{char('C')}, Command: domain.CreateSession{}, Group: "Sessions", Help: "new session"},
			{Name: "rename-session", Keys: []domain.Key{char('$')}, Action: ActionRenameSession, Group: "Sessions", Help: "rename"},
			{Name: "prev-session", Keys: []domain.Key{char('(')}, Action: ActionPrevSession, Group: "Sessions", Help: "prev session"},
			{Name: "next-session", Keys: []domain.Key{char(')')}, Action: ActionNextSession, Group: "Sessions", Help: "next session"},
			{Name: "detach", Keys: []domain.Key{char('d')}, Action: ActionDetach, Group: "Sessions", Help: "detach"},
			{Name: "kill-session", Keys: []domain.Key{char('q')}, Command: domain.KillSession{}, Group: "Sessions", Help: "kill session"},

			{Name: "theme", Keys: []domain.Key{char('t')}, Action: ActionThemePicker, Group: "General", Help: "theme"},
		},
		Arrange: arrangeBindings(),
	}
}

// arrangeBindings are the keys of arrange mode. Moves and resizes go two
// columns or one row at a time, about the same distance on screen.
func arrangeBindings() []Binding {
	move := func(d domain.Direction) domain.Command {
		return domain.MoveWindow{Dir: d, Amount: nudge(d)}
	}
	resize := func(d domain.Direction) domain.Command {
		return domain.ResizeWindow{Dir: d, Amount: nudge(d)}
	}
	return []Binding{
		{Name: "move-left", Keys: []domain.Key{char('h'), special(domain.KeyLeft, 0)}, Command: move(domain.Left), Help: "move"},
		{Name: "move-down", Keys: []domain.Key{char('j'), special(domain.KeyDown, 0)}, Command: move(domain.Down), Help: "move"},
		{Name: "move-up", Keys: []domain.Key{char('k'), special(domain.KeyUp, 0)}, Command: move(domain.Up), Help: "move"},
		{Name: "move-right", Keys: []domain.Key{char('l'), special(domain.KeyRight, 0)}, Command: move(domain.Right), Help: "move"},

		{Name: "resize-left", Keys: []domain.Key{char('H'), special(domain.KeyLeft, domain.ModShift)}, Command: resize(domain.Left), Help: "resize"},
		{Name: "resize-down", Keys: []domain.Key{char('J'), special(domain.KeyDown, domain.ModShift)}, Command: resize(domain.Down), Help: "resize"},
		{Name: "resize-up", Keys: []domain.Key{char('K'), special(domain.KeyUp, domain.ModShift)}, Command: resize(domain.Up), Help: "resize"},
		{Name: "resize-right", Keys: []domain.Key{char('L'), special(domain.KeyRight, domain.ModShift)}, Command: resize(domain.Right), Help: "resize"},

		{Name: "focus-next", Keys: []domain.Key{special(domain.KeyTab, 0)}, Command: domain.CycleFocus{Delta: 1}, Help: "switch"},
		{Name: "focus-prev", Keys: []domain.Key{special(domain.KeyTab, domain.ModShift)}, Command: domain.CycleFocus{Delta: -1}, Help: "switch"},

		{Name: "done", Keys: []domain.Key{special(domain.KeyEscape, 0), special(domain.KeyEnter, 0), char('a'), char('q')}, Action: ActionArrange, Help: "done"},

		{Name: "tidy", Keys: []domain.Key{char('=')}, Command: domain.Tidy{}, Help: "tidy"},
		{Name: "maximize", Keys: []domain.Key{char('z')}, Command: domain.ToggleMaximize{}, Help: "maximize"},
		{Name: "new-window", Keys: []domain.Key{char('c')}, Action: ActionNewWindow, Help: "new window"},
		{Name: "split-right", Keys: []domain.Key{char('|')}, Command: domain.SplitWindow{Dir: domain.Right}, Help: "split right"},
		{Name: "split-down", Keys: []domain.Key{char('-')}, Command: domain.SplitWindow{Dir: domain.Down}, Help: "split down"},
		{Name: "close", Keys: []domain.Key{char('x')}, Command: domain.CloseWindow{}, Help: "close"},
	}
}

func nudge(d domain.Direction) int {
	if d.Axis() == domain.AxisX {
		return 2
	}
	return 1
}
