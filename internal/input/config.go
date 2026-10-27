package input

import (
	"fmt"
	"slices"
	"strings"

	"github.com/thomassbooth/floe/internal/domain"
)

// ConfigError is a problem on one line of a keybinds file.
type ConfigError struct {
	Line int
	Msg  string
}

func (e ConfigError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

// arrangeFlag picks the arrange mode table in bind and unbind.
const arrangeFlag = "--arrange"

// physical is a key without its text, to compare key presses by.
type physical struct {
	code rune
	mod  domain.KeyMod
}

func physicalOf(k domain.Key) physical { return physical{k.Code, k.Mod} }

// ParseConfig applies a keybinds file to km, line by line, in the style of
// tmux:
//
//	# A comment
//	prefix ctrl+a
//	bind v split-right
//	unbind %
//	unbind all
//	bind --arrange w move-up
//
// bind adds a key to a binding and takes it from any other binding in the
// same table, so later lines win. If any line is wrong, none of the file is
// used: km comes back unchanged with every problem found.
func ParseConfig(src string, km Keymap) (Keymap, []ConfigError) {
	out := Keymap{
		Prefix:   km.Prefix,
		Bindings: cloneBindings(km.Bindings),
		Arrange:  cloneBindings(km.Arrange),
	}
	var errs []ConfigError
	// bound is the line that bound each key after the prefix, for keys the
	// file bound itself.
	bound := map[physical]int{}

	for i, line := range strings.Split(src, "\n") {
		n := i + 1
		fail := func(format string, args ...any) {
			errs = append(errs, ConfigError{Line: n, Msg: fmt.Sprintf(format, args...)})
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		cmd, args := fields[0], fields[1:]
		table, arrange := &out.Bindings, false
		if (cmd == "bind" || cmd == "unbind") && len(args) > 0 && args[0] == arrangeFlag {
			table, arrange, args = &out.Arrange, true, args[1:]
		}

		switch cmd {
		case "prefix":
			if len(args) != 1 {
				fail("prefix takes one key, such as: prefix ctrl+a")
				continue
			}
			k, err := domain.ParseKey(args[0])
			if err != nil {
				fail("%v", err)
				continue
			}
			out.Prefix = k

		case "bind":
			if len(args) != 2 {
				fail("bind takes a key and an action, such as: bind v split-right")
				continue
			}
			k, err := domain.ParseKey(args[0])
			if err != nil {
				fail("%v", err)
				continue
			}
			i := slices.IndexFunc(*table, func(b Binding) bool { return b.Name == args[1] })
			if i < 0 {
				fail("%s", unknownAction(args[1], arrange, out))
				continue
			}
			unbindKey(*table, k)
			b := &(*table)[i]
			b.Keys = append([]domain.Key{k}, b.Keys...)
			if !arrange {
				bound[physicalOf(k)] = n
			}

		case "unbind":
			if len(args) != 1 {
				fail("unbind takes a key or all, such as: unbind %%")
				continue
			}
			if args[0] == "all" {
				for i := range *table {
					(*table)[i].Keys = nil
				}
				if !arrange {
					clear(bound)
				}
				continue
			}
			k, err := domain.ParseKey(args[0])
			if err != nil {
				fail("%v", err)
				continue
			}
			unbindKey(*table, k)
			if !arrange {
				delete(bound, physicalOf(k))
			}

		default:
			fail("unknown command %q; use prefix, bind or unbind", cmd)
		}
	}

	// After the prefix, the prefix itself and esc are taken: they send the
	// prefix and cancel. A default binding quietly loses them, but binding
	// them on purpose is a mistake worth pointing out.
	for _, taken := range []struct {
		key domain.Key
		why string
	}{
		{out.Prefix, "is the prefix"},
		{domain.Key{Code: domain.KeyEscape}, "cancels the prefix"},
	} {
		if n, ok := bound[physicalOf(taken.key)]; ok {
			errs = append(errs, ConfigError{Line: n, Msg: fmt.Sprintf("%s %s, so it cannot be bound after it", taken.key, taken.why)})
		}
		unbindKey(out.Bindings, taken.key)
	}

	if len(errs) > 0 {
		slices.SortStableFunc(errs, func(a, b ConfigError) int { return a.Line - b.Line })
		return km, errs
	}
	return out, nil
}

// unknownAction explains a name that is not in the table bind was given,
// pointing to the other table when the name is there.
func unknownAction(name string, arrange bool, km Keymap) string {
	other, hint := km.Arrange, "it is only in arrange mode; use bind --arrange"
	if arrange {
		other, hint = km.Bindings, "it is not in arrange mode; leave out --arrange"
	}
	if slices.ContainsFunc(other, func(b Binding) bool { return b.Name == name }) {
		return fmt.Sprintf("unknown action %q: %s", name, hint)
	}
	return fmt.Sprintf("unknown action %q (floe keys lists them)", name)
}

// unbindKey takes k from every binding in table.
func unbindKey(table []Binding, k domain.Key) {
	for i := range table {
		table[i].Keys = slices.DeleteFunc(table[i].Keys, k.Matches)
	}
}

func cloneBindings(bs []Binding) []Binding {
	out := slices.Clone(bs)
	for i := range out {
		out[i].Keys = slices.Clone(out[i].Keys)
	}
	return out
}
