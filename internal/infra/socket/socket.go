// Package socket is where the floe server listens and how clients find it: a
// Unix socket in a directory only the user can enter, and a lock file the
// server holds for as long as it runs.
package socket

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ErrRunning means another server already holds the lock.
var ErrRunning = errors.New("a floe server is already running")

// ErrNotRunning means no server holds the lock.
var ErrNotRunning = errors.New("no floe server is running")

// DefaultPath returns the server's socket: in $XDG_RUNTIME_DIR/floe, or else
// in floe-<uid> under the temporary directory.
func DefaultPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "floe", "default.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("floe-%d", os.Getuid()), "default.sock")
}

// lockPath is the lock file that goes with the socket at path.
func lockPath(path string) string {
	return strings.TrimSuffix(path, ".sock") + ".lock"
}

// prepareDir makes the socket's directory, or checks an existing one: it
// must belong to the user and let nobody else in, or anyone could talk to
// their shells.
func prepareDir(path string) error {
	if len(path) > maxPath {
		return fmt.Errorf("socket path %s is %d bytes, more than the %d the system allows; set XDG_RUNTIME_DIR to a shorter directory", path, len(path), maxPath)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("make socket directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("check socket directory: %w", err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	switch {
	case !info.IsDir():
		return fmt.Errorf("socket directory %s is not a directory", dir)
	case !ok || int(st.Uid) != os.Getuid():
		return fmt.Errorf("socket directory %s belongs to someone else", dir)
	case info.Mode().Perm()&0o077 != 0:
		return fmt.Errorf("socket directory %s is open to other users (mode %v)", dir, info.Mode().Perm())
	}
	return nil
}

// Lock is held by the running server. The kernel lets go of it if the
// server dies, so a socket without a held lock is known to be stale.
type Lock struct {
	f *os.File
}

// Acquire takes the lock for the socket at path and records this process
// in it. It fails with ErrRunning if another server holds it.
func Acquire(path string) (*Lock, error) {
	if err := prepareDir(path); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath(path), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open server lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrRunning
		}
		return nil, fmt.Errorf("lock server: %w", err)
	}
	if err := f.Truncate(0); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("record server: %w", err)
	}
	if _, err := f.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("record server: %w", err)
	}
	return &Lock{f: f}, nil
}

// Release lets go of the lock.
func (l *Lock) Release() {
	_ = l.f.Close()
}

// Listen listens on the socket at path. The caller must hold its Lock, so
// any socket file already there is stale and is replaced.
func Listen(path string, _ *Lock) (net.Listener, error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("remove stale socket: %w", err)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	return l, nil
}

// Dial connects to the server at path.
func Dial(path string) (net.Conn, error) {
	return net.Dial("unix", path)
}

// ServerPID returns the process ID of the server holding the lock for the
// socket at path, or ErrNotRunning.
func ServerPID(path string) (int, error) {
	f, err := os.Open(lockPath(path))
	if errors.Is(err, os.ErrNotExist) {
		return 0, ErrNotRunning
	}
	if err != nil {
		return 0, fmt.Errorf("open server lock: %w", err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err == nil {
		return 0, ErrNotRunning
	} else if !errors.Is(err, syscall.EWOULDBLOCK) {
		return 0, fmt.Errorf("check server lock: %w", err)
	}
	raw, err := os.ReadFile(f.Name())
	if err != nil {
		return 0, fmt.Errorf("read server lock: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("server lock holds no process ID: %q", raw)
	}
	return pid, nil
}
