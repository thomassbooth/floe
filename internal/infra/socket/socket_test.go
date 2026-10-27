package socket

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// socketPath returns a socket path in a fresh directory. Socket paths are
// short on macOS, so it avoids the long t.TempDir.
func socketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mux")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "sub", "default.sock")
}

func TestOnlyOneServerHoldsTheLock(t *testing.T) {
	path := socketPath(t)
	if _, err := ServerPID(path); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("before any server: %v, want ErrNotRunning", err)
	}

	lock, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(path); !errors.Is(err, ErrRunning) {
		t.Fatalf("second acquire: %v, want ErrRunning", err)
	}
	if pid, err := ServerPID(path); err != nil || pid != os.Getpid() {
		t.Fatalf("server pid = %d, %v; want %d", pid, err, os.Getpid())
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("socket directory mode = %v, %v; want 0700", info.Mode().Perm(), err)
	}

	lock.Release()
	if _, err := ServerPID(path); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("after release: %v, want ErrNotRunning", err)
	}
	again, err := Acquire(path)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	again.Release()
}

func TestListenReplacesAStaleSocket(t *testing.T) {
	path := socketPath(t)
	lock, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	// A server that crashed leaves its socket file behind.
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := Listen(path, lock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	go func() {
		if c, err := l.Accept(); err == nil {
			_ = c.Close()
		}
	}()
	c, err := Dial(path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = c.Close()
}

func TestRefusesADirectoryOthersCanEnter(t *testing.T) {
	path := socketPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(path); err == nil {
		t.Fatal("acquired a lock in a directory open to other users")
	}
}

func TestDefaultPathFollowsXDGRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/7")
	if got := DefaultPath(); got != "/run/user/7/mux/default.sock" {
		t.Fatalf("path = %s", got)
	}
	t.Setenv("XDG_RUNTIME_DIR", "")
	want := filepath.Join(os.TempDir(), fmt.Sprintf("mux-%d", os.Getuid()), "default.sock")
	if got := DefaultPath(); got != want {
		t.Fatalf("path = %s, want %s", got, want)
	}
}

func TestRefusesAPathTooLongForASocket(t *testing.T) {
	path := filepath.Join(socketPath(t), strings.Repeat("d", maxPath), "default.sock")
	_, err := Acquire(path)
	if err == nil || !strings.Contains(err.Error(), "XDG_RUNTIME_DIR") {
		t.Fatalf("err = %v, want advice to shorten the path", err)
	}
}
