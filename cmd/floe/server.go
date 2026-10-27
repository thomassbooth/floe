package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/thomassbooth/floe/internal/infra/socket"
	"github.com/thomassbooth/floe/internal/infra/store"
	"github.com/thomassbooth/floe/internal/infra/terminal"
	"github.com/thomassbooth/floe/internal/remote"
	"github.com/thomassbooth/floe/internal/service"
)

// insideEnv is set in every pane to the server's socket, so floe can tell it
// is being run inside itself.
const insideEnv = "FLOE"

// runServer is the background server: it runs the sessions and serves
// clients until the last session ends or it is told to stop.
func runServer(args []string) error {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	fd := flags.Int("ready-fd", -1, "descriptor to report readiness on")
	if err := flags.Parse(args); err != nil {
		return err
	}
	ready := func(string) {}
	if *fd >= 0 {
		f := os.NewFile(uintptr(*fd), "ready")
		reported := false
		ready = func(line string) {
			if reported {
				return
			}
			reported = true
			_, _ = fmt.Fprintln(f, line)
			_ = f.Close()
		}
		defer ready("") // closing tells the client the server gave up
	}

	// Everything printed goes to the server log; main prints the error.
	log.SetPrefix(fmt.Sprintf("floe server %d: ", os.Getpid()))
	err := serve(ready)
	if err != nil {
		ready(err.Error())
	}
	return err
}

func serve(ready func(string)) error {
	// The server has no terminal; only SIGTERM, from kill-server, stops it.
	// Catch and drop the others rather than ignore them: programs started
	// in panes would inherit ignoring them, and Ctrl+C would not work.
	dropped := make(chan os.Signal, 1)
	signal.Notify(dropped, syscall.SIGHUP, syscall.SIGINT)
	defer signal.Stop(dropped)
	go func() {
		for range dropped {
		}
	}()

	path := socket.DefaultPath()
	lock, err := socket.Acquire(path)
	if errors.Is(err, socket.ErrRunning) {
		ready(readyRunning)
		return nil
	}
	if err != nil {
		return err
	}
	defer lock.Release()
	listener, err := socket.Listen(path, lock)
	if err != nil {
		return err
	}
	defer os.Remove(path)

	build, err := buildID()
	if err != nil {
		return err
	}
	shell, err := terminal.DefaultShell()
	if err != nil {
		return err
	}
	host := terminal.NewHost(terminal.Options{
		Shell: shell,
		Env:   []string{"TERM=xterm-256color", "COLORTERM=truecolor", insideEnv + "=" + path},
	})
	defer host.Close()
	svc := service.New(host, service.Options{
		Title: filepath.Base(shell),
		Store: openStore(),
	})

	runCtx, stopRun := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stopRun()
	ran := make(chan error, 1)
	go func() { ran <- svc.Run(runCtx) }()

	serveCtx, stopServing := context.WithCancel(context.Background())
	defer stopServing()
	srv := remote.NewServer(func() remote.LocalClient { return svc.Connect() }, build)
	served := make(chan error, 1)
	go func() { served <- srv.Serve(serveCtx, listener) }()

	log.Print("listening on ", path)
	ready(readyListening)

	// The service stops when the last session ends or on SIGTERM; then
	// serving stops, once clients have heard. If serving fails first, stop
	// the service, which saves sessions on the way out.
	var runErr, serveErr error
	select {
	case runErr = <-ran:
		stopServing()
		serveErr = <-served
	case serveErr = <-served:
		stopRun()
		runErr = <-ran
	}
	log.Print("stopped")
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return runErr
	}
	return serveErr
}

// openStore opens the sessions file. Without one, sessions are simply not
// kept between runs.
func openStore() service.SessionStore {
	path, err := store.DefaultPath()
	if err != nil {
		log.Print("sessions will not be saved: ", err)
		return nil
	}
	return store.New(path)
}

// serverLogPath is where the server writes what it would otherwise print,
// next to the sessions file.
func serverLogPath() (string, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return "", fmt.Errorf("find the server log: %w", err)
	}
	return filepath.Join(filepath.Dir(path), "server.log"), nil
}

// openServerLog starts a fresh log for a new server.
func openServerLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("open the server log: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open the server log: %w", err)
	}
	return f, nil
}
