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
			{Keys: []domain.Key{char('|'), char('%')}, Command: domain.SplitWindow{Dir: domain.Right}, Help: "split right"},
			{Keys: []domain.Key{char('-'), char('"')}, Command: domain.SplitWindow{Dir: domain.Down}, Help: "split down"},

			{Keys: []domain.Key{char('h'), special(domain.KeyLeft, 0)}, Command: domain.FocusDirection{Dir: domain.Left}, Help: "focus left"},
			{Keys: []domain.Key{char('j'), special(domain.KeyDown, 0)}, Command: domain.FocusDirection{Dir: domain.Down}, Help: "focus down"},
			{Keys: []domain.Key{char('k'), special(domain.KeyUp, 0)}, Command: domain.FocusDirection{Dir: domain.Up}, Help: "focus up"},
			{Keys: []domain.Key{char('l'), special(domain.KeyRight, 0)}, Command: domain.FocusDirection{Dir: domain.Right}, Help: "focus right"},

			{Keys: []domain.Key{char('H'), special(domain.KeyLeft, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Left, Amount: resizeStep}, Help: "resize left", Repeat: true},
			{Keys: []domain.Key{char('J'), special(domain.KeyDown, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Down, Amount: resizeStep}, Help: "resize down", Repeat: true},
			{Keys: []domain.Key{char('K'), special(domain.KeyUp, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Up, Amount: resizeStep}, Help: "resize up", Repeat: true},
			{Keys: []domain.Key{char('L'), special(domain.KeyRight, domain.ModShift)}, Command: domain.ResizeWindow{Dir: domain.Right, Amount: resizeStep}, Help: "resize right", Repeat: true},

			{Keys: []domain.Key{char('x')}, Command: domain.CloseWindow{}, Help: "close"},
			{Keys: []domain.Key{char('q')}, Command: domain.KillSession{}, Help: "quit"},
		},
	}
}
