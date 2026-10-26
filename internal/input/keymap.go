package input

import "github.com/thomasbooth/multiplexer/internal/domain"

// resizeStep is how many cells one resize key press moves a divider.
const resizeStep = 2

func char(r rune) domain.Key { return domain.Key{Code: r, Text: string(r)} }

func special(r rune, mod domain.KeyMod) domain.Key { return domain.Key{Code: r, Mod: mod} }

// DefaultKeymap returns tmux-like bindings behind ctrl+b.
func DefaultKeymap() Keymap {
	return Keymap{
		Prefix: domain.Key{Code: 'b', Mod: domain.ModCtrl},
		Bindings: []Binding{
			{Keys: []domain.Key{char('c')}, Action: ActionNewWindow, Group: "Windows", Help: "new window"},
			{Keys: []domain.Key{char('|'), char('%')}, Command: domain.SplitWindow{Dir: domain.Right}, Group: "Windows", Help: "split right"},
			{Keys: []domain.Key{char('-'), char('"')}, Command: domain.SplitWindow{Dir: domain.Down}, Group: "Windows", Help: "split down"},

			{Keys: []domain.Key{char('h'), special(domain.KeyLeft, 0)}, Command: domain.FocusDirection{Dir: domain.Left}, Group: "Windows", Help: "focus left"},
			{Keys: []domain.Key{char('j'), special(domain.KeyDown, 0)}, Command: domain.FocusDirection{Dir: domain.Down}, Group: "Windows", Help: "focus down"},
			{Keys: []domain.Key{char('k'), special(domain.KeyUp, 0)}, Command: domain.FocusDirection{Dir: domain.Up}, Group: "Windows", Help: "focus up"},
			{Keys: []domain.Key{char('l'), special(domain.KeyRight, 0)}, Command: domain.FocusDirection{Dir: domain.Right}, Group: "Windows", Help: "focus right"},

			{Keys: []domain.Key{char('H'), special(domain.KeyLeft, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Left, Amount: resizeStep}, Group: "Windows", Help: "resize left", Repeat: true},
			{Keys: []domain.Key{char('J'), special(domain.KeyDown, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Down, Amount: resizeStep}, Group: "Windows", Help: "resize down", Repeat: true},
			{Keys: []domain.Key{char('K'), special(domain.KeyUp, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Up, Amount: resizeStep}, Group: "Windows", Help: "resize up", Repeat: true},
			{Keys: []domain.Key{char('L'), special(domain.KeyRight, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Right, Amount: resizeStep}, Group: "Windows", Help: "resize right", Repeat: true},

			{Keys: []domain.Key{char('=')}, Command: domain.Tidy{}, Group: "Windows", Help: "tidy"},
			{Keys: []domain.Key{char('z')}, Command: domain.ToggleMaximize{}, Group: "Windows", Help: "maximize"},
			{Keys: []domain.Key{char('x')}, Command: domain.CloseWindow{}, Group: "Windows", Help: "close"},

			{Keys: []domain.Key{char('s')}, Action: ActionSessionSwitcher, Group: "Sessions", Help: "sessions"},
			{Keys: []domain.Key{char('C')}, Command: domain.CreateSession{}, Group: "Sessions", Help: "new session"},
			{Keys: []domain.Key{char('$')}, Action: ActionRenameSession, Group: "Sessions", Help: "rename"},
			{Keys: []domain.Key{char('(')}, Action: ActionPrevSession, Group: "Sessions", Help: "prev session"},
			{Keys: []domain.Key{char(')')}, Action: ActionNextSession, Group: "Sessions", Help: "next session"},
			{Keys: []domain.Key{char('q')}, Command: domain.KillSession{}, Group: "Sessions", Help: "kill session"},
		},
	}
}
