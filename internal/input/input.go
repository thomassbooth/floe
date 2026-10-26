// Package input turns key presses into commands. It is a small state machine
// driven by a keymap: keys go to the focused pane until the prefix key is
// pressed, and the key after the prefix picks a binding.
package input

import "github.com/thomasbooth/multiplexer/internal/domain"

// Mode is the state of the input machine.
type Mode int

const (
	// Normal sends keys to the focused pane.
	Normal Mode = iota
	// Prefix waits for a binding after the prefix key.
	Prefix
	// Repeat follows a repeatable binding: further repeatable bindings run
	// without pressing the prefix again, and any other key returns to Normal.
	Repeat
)

// Binding maps a key pressed after the prefix to a command.
type Binding struct {
	Keys    []domain.Key
	Command domain.Command
	// Help is a short description, such as "split right".
	Help string
	// Repeat lets the binding be pressed again without the prefix.
	Repeat bool
}

// Keymap is the full set of bindings.
type Keymap struct {
	Prefix   domain.Key
	Bindings []Binding
}

// Machine tracks the current mode and maps keys to commands.
type Machine struct {
	keymap Keymap
	mode   Mode
}

// NewMachine returns a machine in Normal mode.
func NewMachine(km Keymap) *Machine {
	return &Machine{keymap: km}
}

// Mode returns the current mode.
func (m *Machine) Mode() Mode { return m.mode }

// Keymap returns the bindings the machine uses.
func (m *Machine) Keymap() Keymap { return m.keymap }

// Handle processes a key and returns the command it produces, if any.
func (m *Machine) Handle(k domain.Key) (domain.Command, bool) {
	switch m.mode {
	case Prefix:
		m.mode = Normal
		switch {
		case k.Matches(m.keymap.Prefix):
			return domain.SendKey{Key: k}, true
		case k.Code == domain.KeyEscape && k.Mod == 0:
			return nil, false
		}
		b, ok := m.lookup(k)
		if !ok {
			return nil, false
		}
		if b.Repeat {
			m.mode = Repeat
		}
		return b.Command, true

	case Repeat:
		if b, ok := m.lookup(k); ok && b.Repeat {
			return b.Command, true
		}
		m.mode = Normal
		return m.Handle(k)
	}

	if k.Matches(m.keymap.Prefix) {
		m.mode = Prefix
		return nil, false
	}
	return domain.SendKey{Key: k}, true
}

// Reset returns to Normal mode, for example when focus moves by mouse.
func (m *Machine) Reset() { m.mode = Normal }

func (m *Machine) lookup(k domain.Key) (Binding, bool) {
	for _, b := range m.keymap.Bindings {
		for _, bk := range b.Keys {
			if k.Matches(bk) {
				return b, true
			}
		}
	}
	return Binding{}, false
}
