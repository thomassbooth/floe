package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	tea "charm.land/bubbletea/v2"

	"github.com/thomassbooth/floe/internal/domain"
	"github.com/thomassbooth/floe/internal/infra/settings"
	"github.com/thomassbooth/floe/internal/infra/socket"
	"github.com/thomassbooth/floe/internal/remote"
	"github.com/thomassbooth/floe/internal/ui"
)

// otherBuild explains what to do when the server is older, or newer, than
// this client.
func otherBuild(server string) error {
	what := fmt.Sprintf("is floe %s, but this is floe %s", server, release())
	if server == release() {
		what = fmt.Sprintf("is a different build of floe %s", server)
	}
	return fmt.Errorf(`the running floe server %s.
Run "floe kill-server" to replace it: windows and layouts are kept, but the programs in them stop`, what)
}

// runClient attaches this terminal to the server and shows it until the
// client detaches.
func runClient() error {
	if os.Getenv(insideEnv) != "" {
		return fmt.Errorf("already inside floe; unset %s to run floe inside itself", insideEnv)
	}
	build, err := buildID()
	if err != nil {
		return err
	}
	path := socket.DefaultPath()
	conn, err := connect(path)
	if err != nil {
		return err
	}
	client, err := remote.Dial(conn, build)
	var other *remote.OtherBuildError
	if errors.As(err, &other) {
		return otherBuild(other.Release)
	}
	if err != nil {
		return err
	}
	defer client.Close()

	opts := clientOptions()
	client.Apply(domain.AttachSession{})
	program := tea.NewProgram(ui.New(client, opts))

	// Closing the terminal sends SIGHUP. The server keeps everything
	// running; just stop showing it.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM)
	defer signal.Stop(signals)
	stopped := make(chan struct{})
	defer close(stopped)
	var signalled atomic.Bool
	go func() {
		select {
		case <-signals:
			signalled.Store(true)
			program.Kill()
		case <-stopped:
		}
	}()

	final, err := program.Run()
	if err != nil && !(signalled.Load() && errors.Is(err, tea.ErrProgramKilled)) {
		return err
	}
	if m, ok := final.(ui.Model); ok {
		if m.Err != nil {
			return m.Err
		}
		if m.DetachedFrom != "" {
			fmt.Printf("[detached from session %s]\n", m.DetachedFrom)
		}
	}
	return nil
}

// openPreferences loads the settings file. If it cannot be read, floe still
// starts, with nothing remembered, and leaves the file alone.
func openPreferences() ui.Preferences {
	path, err := settings.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "floe: settings not available:", err)
		return nil
	}
	f, err := settings.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "floe: settings not available:", err)
		return nil
	}
	return f
}
