package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/thomassbooth/floe/internal/infra/settings"
	"github.com/thomassbooth/floe/internal/input"
)

// loadKeymap returns the default keymap with keybinds.conf applied. When the
// file is missing the defaults are all there is. When it is broken the
// defaults are used and every problem comes back, each naming the file and
// line, so floe still starts and says what to fix.
func loadKeymap() (input.Keymap, []error) {
	km := input.DefaultKeymap()
	path, err := settings.KeybindsPath()
	if err != nil {
		return km, []error{err}
	}
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return km, nil
	}
	if err != nil {
		return km, []error{err}
	}
	km, configErrs := input.ParseConfig(string(src), km)
	errs := make([]error, len(configErrs))
	for i, e := range configErrs {
		errs[i] = fmt.Errorf("%s %w", filepath.Base(path), e)
	}
	return km, errs
}

// keymapNotice sums up keymap problems for the status bar, which has room
// for one line.
func keymapNotice(errs []error) string {
	switch len(errs) {
	case 0:
		return ""
	case 1:
		return errs[0].Error() + " · using the default keys"
	}
	return fmt.Sprintf("%s · %d more in floe keys · using the default keys", errs[0], len(errs)-1)
}

// listKeys prints every binding with its name and keys, as floe keys. It
// fails if keybinds.conf has problems, after listing them.
func listKeys(w io.Writer) error {
	km, errs := loadKeymap()
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintln(w, "floe:", err)
		}
		return errors.New("fix keybinds.conf to see your keys")
	}
	path, _ := settings.KeybindsPath()
	fmt.Fprintf(w, "# Change these in %s.\n\n", path)
	fmt.Fprintf(w, "prefix %s\n\n", km.Prefix)
	fmt.Fprintln(w, "# After the prefix: bind <key> <action>")
	writeBindings(w, km.Bindings)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "# In arrange mode: bind --arrange <key> <action>")
	writeBindings(w, km.Arrange)
	return nil
}

func writeBindings(w io.Writer, bindings []input.Binding) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	for _, b := range bindings {
		keys := make([]string, len(b.Keys))
		for i, k := range b.Keys {
			keys[i] = k.String()
		}
		if len(keys) == 0 {
			keys = []string{"(none)"}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", b.Name, strings.Join(keys, " "), b.Help)
	}
	tw.Flush()
}
