package terminal

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// processName returns the short command name of a process.
func processName(pid int) (string, bool) {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || kp.Proc.P_pid != int32(pid) {
		return "", false
	}
	// The name is NUL-terminated inside a fixed buffer that may hold
	// leftover bytes after the terminator.
	comm := kp.Proc.P_comm[:]
	if i := bytes.IndexByte(comm, 0); i >= 0 {
		comm = comm[:i]
	}
	name := string(comm)
	return name, name != ""
}

// workingDir returns a process's current directory. macOS only exposes it
// through proc_pidinfo, which needs cgo, so this asks lsof, which ships with
// the system.
func workingDir(pid int) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if dir, ok := strings.CutPrefix(line, "n"); ok && dir != "" {
			return dir, true
		}
	}
	return "", false
}
