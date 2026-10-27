package ui

// Preferences keeps the client's choices across restarts. The settings file
// implements it; a nil Preferences remembers nothing.
type Preferences interface {
	Theme() (string, bool)
	SetTheme(name string) error
}
