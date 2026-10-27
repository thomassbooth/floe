package domain

// Command is a request from a client. Keys, mouse actions and any future CLI
// or config all produce commands, and the service applies them. Window
// commands act on the session the client is attached to.
type Command interface {
	command()
}

// Result is what came of a command, for a client that asked to hear.
type Result struct {
	// Err is set when the command could not be carried out.
	Err error
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

// MoveWindow shifts the focused window Amount cells in direction Dir.
type MoveWindow struct {
	Dir    Direction
	Amount int
}

// CycleFocus moves focus Delta windows along in window order, wrapping
// around: 1 is the next window, -1 the previous.
type CycleFocus struct {
	Delta int
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

// MinimizeWindow hides a window and leaves its program running. A zero ID
// means the focused window. Focusing the window shows it again.
type MinimizeWindow struct {
	ID WindowID
}

// RestoreMinimized shows and focuses the most recently minimized window.
type RestoreMinimized struct{}

// SnapWindow moves a window into a zone of the screen, remembering where it
// was.
type SnapWindow struct {
	ID   WindowID
	Zone Zone
}

// Tidy snaps every window into a layout that fills the screen.
type Tidy struct{}

// CloseWindow closes a window and ends its pane's process. A zero ID means
// the focused window.
type CloseWindow struct {
	ID WindowID
}

// SendKey forwards a key press to a window's pane: the focused window when
// Window is zero.
type SendKey struct {
	Key    Key
	Window WindowID
}

// SendMouse forwards a mouse event to a window's pane, with X and Y
// relative to the window's content.
type SendMouse struct {
	Window WindowID
	Event  MouseEvent
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
func (MoveWindow) command()       {}
func (CycleFocus) command()       {}
func (PlaceWindow) command()      {}
func (ResizeWindowEdge) command() {}
func (SnapWindow) command()       {}
func (Tidy) command()             {}
func (ToggleMaximize) command()   {}
func (MinimizeWindow) command()   {}
func (RestoreMinimized) command() {}
func (CloseWindow) command()      {}
func (KillSession) command()      {}
func (SendMouse) command()        {}
func (SendKey) command()          {}
func (Paste) command()            {}
