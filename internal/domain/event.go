package domain

// Event is something the server tells a client.
type Event interface {
	event()
}

// LayoutChanged carries the new state of the client's session after windows
// were added, removed, moved, resized or refocused, or the client switched
// sessions.
type LayoutChanged struct {
	Snapshot Snapshot
}

// SessionsChanged lists every session after one was created, renamed,
// removed or changed its windows.
type SessionsChanged struct {
	Sessions []SessionInfo
	// Current is the session the client is attached to.
	Current SessionID
}

// ClipboardSet asks the client to put Text on its clipboard, because a
// program in the client's session copied it.
type ClipboardSet struct {
	Text string
}

// CommandFailed reports a command that could not be carried out, when the
// client sent it without asking for its result. It also reports problems
// on the server that no command caused, such as a session failing to
// start; Command is nil then.
type CommandFailed struct {
	Command Command
	Err     error
}

// Detached is the last event a client receives: it is no longer attached to
// any session. Err is set when this happened because of a failure.
type Detached struct {
	Err error
}

func (LayoutChanged) event()   {}
func (SessionsChanged) event() {}
func (ClipboardSet) event()    {}
func (CommandFailed) event()   {}
func (Detached) event()        {}
