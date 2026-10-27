package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/thomassbooth/floe/internal/infra/socket"
)

const (
	// startTimeout is how long a new server has to say it is ready.
	startTimeout = 10 * time.Second
	// readyFD is the descriptor a new server reports readiness on.
	readyFD = 3
)

// Readiness reports, one line each, that a new server sent its client.
const (
	readyListening = "listening"
	readyRunning   = "running"
)

// connect reaches the server at path, starting one if none is running.
func connect(path string) (net.Conn, error) {
	if conn, err := socket.Dial(path); err == nil {
		return conn, nil
	}
	if err := startServer(); err != nil {
		return nil, err
	}
	// The server is listening, or another client just started one that
	// is about to; give it a moment.
	deadline := time.Now().Add(startTimeout)
	for {
		conn, err := socket.Dial(path)
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("connect to the floe server: %w", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// startServer runs "floe server" in the background, in a session of its own
// so closing this terminal does not stop it, and waits until it is ready.
func startServer() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("start the floe server: %w", err)
	}
	logPath, err := serverLogPath()
	if err != nil {
		return err
	}
	logFile, err := openServerLog(logPath)
	if err != nil {
		return err
	}
	defer logFile.Close()

	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("start the floe server: %w", err)
	}
	defer r.Close()

	cmd := exec.Command(exe, "server", fmt.Sprintf("--ready-fd=%d", readyFD))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.ExtraFiles = []*os.File{w} // becomes readyFD
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	err = cmd.Start()
	_ = w.Close()
	if err != nil {
		return fmt.Errorf("start the floe server: %w", err)
	}
	// Reap the server if it ends while this client still runs.
	go func() { _ = cmd.Wait() }()

	_ = r.SetReadDeadline(time.Now().Add(startTimeout))
	report, err := io.ReadAll(r)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return fmt.Errorf("the floe server did not start in time; see %s", logPath)
	}
	switch line := strings.TrimSpace(string(report)); line {
	case readyListening, readyRunning:
		return nil
	case "":
		return fmt.Errorf("the floe server stopped while starting; see %s", logPath)
	default:
		return fmt.Errorf("the floe server could not start: %s", line)
	}
}
