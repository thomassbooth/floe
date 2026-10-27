package main

import (
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/thomassbooth/floe/internal/infra/socket"
)

// stopTimeout is how long the server gets to save and stop.
const stopTimeout = 10 * time.Second

// killServer stops the server, whatever build it is: sessions are saved,
// then every program in them stops.
func killServer() error {
	path := socket.DefaultPath()
	pid, err := socket.ServerPID(path)
	if err != nil {
		return err
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("stop the floe server: %w", err)
	}
	deadline := time.Now().Add(stopTimeout)
	for time.Now().Before(deadline) {
		if _, err := socket.ServerPID(path); errors.Is(err, socket.ErrNotRunning) {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("the floe server (process %d) did not stop", pid)
}
