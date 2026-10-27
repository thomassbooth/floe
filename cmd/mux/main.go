// Command mux is a terminal multiplexer.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"

	tea "charm.land/bubbletea/v2"

	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/infra/settings"
	"github.com/thomasbooth/multiplexer/internal/infra/store"
	"github.com/thomasbooth/multiplexer/internal/infra/terminal"
	"github.com/thomasbooth/multiplexer/internal/input"
	"github.com/thomasbooth/multiplexer/internal/service"
	"github.com/thomasbooth/multiplexer/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mux:", err)
		os.Exit(1)
	}
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

// openStore opens the sessions file. Without one, sessions are simply not
// kept between runs.
func openStore() service.SessionStore {
	path, err := store.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mux: sessions will not be saved:", err)
		return nil
	}
	return store.New(path)
}

func run() error {
	shell, err := terminal.DefaultShell()
	if err != nil {
		return err
	}

	host := terminal.NewHost(terminal.Options{
		Shell: shell,
		Env:   []string{"TERM=xterm-256color", "COLORTERM=truecolor"},
	})
	defer host.Close()
	svc := service.New(host, service.Options{
		Title: filepath.Base(shell),
		Store: openStore(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	prefs := openPreferences()

	client := svc.Connect()
	client.Apply(domain.AttachSession{})
	program := tea.NewProgram(ui.New(client, input.DefaultKeymap(), prefs))

	// Closing the terminal sends SIGHUP. Stop the UI on it, and on SIGTERM,
	// so the service still saves sessions before the shells stop.
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
	client.Close()
	cancel()
	runErr := <-done
	if err != nil && !(signalled.Load() && errors.Is(err, tea.ErrProgramKilled)) {
		return err
	}
	if m, ok := final.(ui.Model); ok && m.Err != nil {
		return m.Err
	}
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return runErr
	}
	return nil
}
