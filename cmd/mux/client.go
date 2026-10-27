package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	tea "charm.land/bubbletea/v2"

	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/infra/settings"
	"github.com/thomasbooth/multiplexer/internal/infra/socket"
	"github.com/thomasbooth/multiplexer/internal/input"
	"github.com/thomasbooth/multiplexer/internal/remote"
	"github.com/thomasbooth/multiplexer/internal/ui"
)

// errOtherBuild explains what to do when the server is older, or newer,
// than this client.
var errOtherBuild = errors.New(`the running mux server is a different build of mux.
Run "mux kill-server" to replace it: windows and layouts are kept, but the programs in them stop`)

// runClient attaches this terminal to the server and shows it until the
// client detaches.
func runClient() error {
	if os.Getenv(insideEnv) != "" {
		return fmt.Errorf("already inside mux; unset %s to run mux inside itself", insideEnv)
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
	if errors.Is(err, remote.ErrOtherBuild) {
		return errOtherBuild
	}
	if err != nil {
		return err
	}
	defer client.Close()

	client.Apply(domain.AttachSession{})
	program := tea.NewProgram(ui.New(client, input.DefaultKeymap(), openPreferences()))

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
	if m, ok := final.(ui.Model); ok && m.Err != nil {
		return m.Err
	}
	return nil
}

// openPreferences loads the settings file. If it cannot be read, mux still
// starts, with nothing remembered, and leaves the file alone.
func openPreferences() ui.Preferences {
	path, err := settings.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mux: settings not available:", err)
		return nil
	}
	f, err := settings.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mux: settings not available:", err)
		return nil
	}
	return f
}
