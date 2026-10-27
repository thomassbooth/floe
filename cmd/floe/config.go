package main

import (
	"fmt"
	"os"

	"github.com/thomassbooth/floe/internal/infra/settings"
	"github.com/thomassbooth/floe/internal/ui"
)

// clientOptions gathers what the client needs from the config files. Any
// problem with them comes back as a notice for the status bar, and floe
// starts anyway with the defaults for that file.
func clientOptions() ui.Options {
	km, errs := loadKeymap()
	cfg, cfgErrs := loadConfig()
	errs = append(errs, cfgErrs...)

	animations := !overSSH()
	if cfg.Animations != nil {
		animations = *cfg.Animations
	}
	return ui.Options{
		Keymap:     km,
		Prefs:      openPreferences(),
		Animations: animations,
		Notice:     configNotice(errs),
	}
}

// loadConfig reads floe.conf, if there is one.
func loadConfig() (settings.Config, []error) {
	path, err := settings.ConfigPath()
	if err != nil {
		return settings.Config{}, []error{err}
	}
	return settings.ReadConfig(path)
}

// overSSH reports whether floe is being used over SSH, where each frame of
// an animation is a round trip that may arrive late or in a lump.
func overSSH() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != ""
}

// configNotice sums up problems with the config files for the status bar,
// which has room for one line.
func configNotice(errs []error) string {
	switch len(errs) {
	case 0:
		return ""
	case 1:
		return errs[0].Error() + " · using the defaults until it is fixed"
	}
	return fmt.Sprintf("%s · %d more · using the defaults until they are fixed", errs[0], len(errs)-1)
}
