package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// buildID names this floe binary by its contents, so a client never talks to
// a server built from different code, even one built moments ago.
func buildID() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find floe binary: %w", err)
	}
	f, err := os.Open(exe)
	if err != nil {
		return "", fmt.Errorf("read floe binary: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("read floe binary: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)[:16]), nil
}
