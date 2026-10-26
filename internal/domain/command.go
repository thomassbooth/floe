package domain

// Command is a request to change the session. Keys, mouse actions and any
// future CLI or config all produce commands, and the service applies them.
type Command interface {
	command()
}

// SetArea tells the session how much screen space it may lay windows out in.
type SetArea struct {
	Size Size
}

// SplitWindow splits the focused window, opening a new one on the Dir side.
type SplitWindow struct {
	Dir Direction
}

// FocusDirection moves focus to the window next to the focused one.
type FocusDirection struct {
	Dir Direction
}

// FocusWindow moves focus to a specific window.
type FocusWindow struct {
	ID WindowID
}

// ResizeWindow moves the nearest divider of the focused window by Amount
// cells in direction Dir.
type ResizeWindow struct {
	Dir    Direction
	Amount int
}

// CloseWindow closes the focused window and ends its pane's process.
type CloseWindow struct{}

// KillSession closes every window, ending the session.
type KillSession struct{}

// SendKey forwards a key press to the focused pane.
type SendKey struct {
	Key Key
}

// Paste forwards pasted text to the focused pane.
type Paste struct {
	Text string
}

func (SetArea) command()        {}
func (SplitWindow) command()    {}
func (FocusDirection) command() {}
func (FocusWindow) command()    {}
func (ResizeWindow) command()   {}
func (CloseWindow) command()    {}
func (KillSession) command()    {}
func (SendKey) command()        {}
func (Paste) command()          {}
