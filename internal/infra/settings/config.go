package settings

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Config is what people write in floe.conf. Unset fields are nil, so the
// client can pick a default that suits where it runs.
type Config struct {
	// Animations moves windows smoothly when the layout changes.
	Animations *bool
}

// ConfigPath returns where general settings live: floe.conf in Dir.
func ConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "floe.conf"), nil
}

// ReadConfig reads the floe.conf at path. A missing file is an empty
// config. A file with any mistake is not used at all: it comes back empty
// with every problem found, each naming the file and line.
func ReadConfig(path string) (Config, []error) {
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, []error{fmt.Errorf("read config: %w", err)}
	}
	cfg, errs := ParseConfig(string(src))
	for i, e := range errs {
		errs[i] = fmt.Errorf("%s %w", filepath.Base(path), e)
	}
	if len(errs) > 0 {
		return Config{}, errs
	}
	return cfg, nil
}

// ParseConfig reads the text of a floe.conf: one setting per line, its name
// and value separated by spaces, and lines starting with # are comments.
//
//	# Windows jump into place.
//	animations off
func ParseConfig(src string) (Config, []error) {
	var cfg Config
	var errs []error
	for i, line := range strings.Split(src, "\n") {
		fail := func(format string, args ...any) {
			errs = append(errs, fmt.Errorf("line %d: %s", i+1, fmt.Sprintf(format, args...)))
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		name, args := fields[0], fields[1:]
		switch name {
		case "animations":
			on, ok := onOff(args)
			if !ok {
				fail("animations takes on or off")
				continue
			}
			cfg.Animations = &on
		default:
			fail("unknown setting %q", name)
		}
	}
	return cfg, errs
}

func onOff(args []string) (on, ok bool) {
	if len(args) != 1 {
		return false, false
	}
	switch args[0] {
	case "on":
		return true, true
	case "off":
		return false, true
	}
	return false, false
}
