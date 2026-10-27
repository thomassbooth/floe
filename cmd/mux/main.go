// Command mux is a terminal multiplexer.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/infra/settings"
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

func run() error {
	shell, err := terminal.DefaultShell()
	if err != nil {
		return err
	}

	host := terminal.NewHost(terminal.Options{
		Shell: shell,
		Env:   []string{"TERM=xterm-256color", "COLORTERM=truecolor"},
	})
	svc := service.New(host, service.Options{Title: filepath.Base(shell)})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	prefs := openPreferences()

	client := svc.Connect()
	client.Apply(domain.AttachSession{})
	final, err := tea.NewProgram(ui.New(client, input.DefaultKeymap(), prefs)).Run()
	client.Close()
	cancel()
	runErr := <-done
	if err != nil {
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
