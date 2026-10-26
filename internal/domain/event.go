package domain

// Event is something the session announces to its clients.
type Event interface {
	event()
}

// LayoutChanged carries the session's new state after windows were added,
// removed, moved, resized or refocused.
type LayoutChanged struct {
	Snapshot Snapshot
}

// CommandFailed reports a command that could not be carried out.
type CommandFailed struct {
	Command Command
	Err     error
}

// SessionEnded is the last event a session sends. Err is set when the
// session ended because of a failure rather than its windows closing.
type SessionEnded struct {
	Err error
}

func (LayoutChanged) event() {}
func (CommandFailed) event() {}
func (SessionEnded) event()  {}
