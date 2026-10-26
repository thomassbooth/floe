package domain

// Command is a request from a client. Keys, mouse actions and any future CLI
// or config all produce commands, and the service applies them. Window
// commands act on the session the client is attached to.
type Command interface {
	command()
}

// SetArea tells the server how much screen space the client has for
// windows.
type SetArea struct {
	Size Size
}

// AttachSession attaches the client to the session called Name. An empty
// Name picks the most recently used session, creating one if there are none.
type AttachSession struct {
	Name string
}

// CreateSession creates a session and attaches the client to it. An empty Name
// picks a number.
type CreateSession struct {
	Name string
}

// SwitchSession attaches the client to another session.
type SwitchSession struct {
	ID SessionID
}

// RenameSession renames the client's session.
type RenameSession struct {
	Name string
}

// KillSession ends a session and every window in it. Clients viewing it move
// to the most recently used other session, or detach if there is none. A
// zero ID means the client's own session.
type KillSession struct {
	ID SessionID
}

// SplitWindow splits the focused window, opening a new one on the Dir side.
type SplitWindow struct {
	Dir Direction
}

// NewWindow opens a window at Frame on top of the others, without splitting
// any. The frame is kept on screen.
type NewWindow struct {
	Frame Rect
}

// FocusDirection moves focus to the window next to the focused one.
type FocusDirection struct {
	Dir Direction
}

// FocusWindow moves focus to a specific window.
type FocusWindow struct {
	ID WindowID
}

// ResizeWindow grows (Right, Down) or shrinks (Left, Up) the focused window
// by Amount cells.
type ResizeWindow struct {
	Dir    Direction
	Amount int
}

// PlaceWindow moves and sizes a window. It is kept on screen.
type PlaceWindow struct {
	ID    WindowID
	Frame Rect
}

// ResizeWindowEdge moves one edge of a window to Position, leaving the
// opposite edge in place. Left and Up edges sit at Position; Right and Down
// edges end just before it.
type ResizeWindowEdge struct {
	ID       WindowID
	Edge     Direction
	Position int
}

// ToggleMaximize makes a window fill the screen, or puts it back. A zero ID
// means the focused window.
type ToggleMaximize struct {
	ID WindowID
}

// CloseWindow closes a window and ends its pane's process. A zero ID means
// the focused window.
type CloseWindow struct {
	ID WindowID
}

// SendKey forwards a key press to the focused pane.
type SendKey struct {
	Key Key
}

// Paste forwards pasted text to the focused pane.
type Paste struct {
	Text string
}

func (SetArea) command()          {}
func (AttachSession) command()    {}
func (CreateSession) command()    {}
func (SwitchSession) command()    {}
func (RenameSession) command()    {}
func (NewWindow) command()        {}
func (SplitWindow) command()      {}
func (FocusDirection) command()   {}
func (FocusWindow) command()      {}
func (ResizeWindow) command()     {}
func (PlaceWindow) command()      {}
func (ResizeWindowEdge) command() {}
func (ToggleMaximize) command()   {}
func (CloseWindow) command()      {}
func (KillSession) command()      {}
func (SendKey) command()          {}
func (Paste) command()            {}
