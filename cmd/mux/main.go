// Command mux is a terminal multiplexer.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

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

	final, err := tea.NewProgram(ui.New(svc, input.DefaultKeymap())).Run()
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
