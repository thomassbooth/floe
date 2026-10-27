package terminal

import (
	"os"
	"strconv"
	"strings"
)

// processName returns the short command name of a process.
func processName(pid int) (string, bool) {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return "", false
	}
	name := strings.TrimSpace(string(raw))
	return name, name != ""
}

// workingDir returns a process's current directory.
func workingDir(pid int) (string, bool) {
	dir, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
	return dir, err == nil
}
