package terminal

import (
	"os"

	"golang.org/x/sys/unix"
)

// foreground returns the process group in the foreground of a pane's
// terminal: the shell at a prompt, or the program it is running. It goes
// through SyscallConn rather than Fd, which would switch the file to
// blocking mode and stop Close from interrupting a pending read.
func foreground(pty *os.File) (int, bool) {
	conn, err := pty.SyscallConn()
	if err != nil {
		return 0, false
	}
	var pgid int
	var ioctlErr error
	if err := conn.Control(func(fd uintptr) {
		pgid, ioctlErr = unix.IoctlGetInt(int(fd), unix.TIOCGPGRP)
	}); err != nil || ioctlErr != nil || pgid <= 0 {
		return 0, false
	}
	return pgid, true
}
