package service

import "github.com/thomassbooth/floe/internal/domain"

// PaneHost runs the processes behind panes. The infrastructure layer
// implements it.
type PaneHost interface {
	// Spawn starts a pane's process with a screen of the given size, in
	// directory dir, or a default directory when dir is empty.
	Spawn(id domain.PaneID, size domain.Size, dir string) error
	// WorkingDir returns the directory a pane's process is in.
	WorkingDir(id domain.PaneID) (string, bool)
	Resize(id domain.PaneID, size domain.Size)
	SendKey(id domain.PaneID, key domain.Key)
	Paste(id domain.PaneID, text string)
	// SendMouse passes a mouse event to a pane's program, if it asked for
	// that kind of event.
	SendMouse(id domain.PaneID, ev domain.MouseEvent)
	// Kill ends a pane's process. Its exit is still reported on Exited.
	Kill(id domain.PaneID)
	// Grid copies a pane's screen, scrolled scroll lines up into its
	// history.
	Grid(id domain.PaneID, scroll int) (domain.Grid, bool)
	// State describes a pane's history and input modes.
	State(id domain.PaneID) (domain.PaneState, bool)
	// Exited receives the ID of every pane whose process ended.
	Exited() <-chan domain.PaneID
	// Changed receives a signal when any pane's screen changes. Signals
	// coalesce, so one receive may stand for many changes; TakeChanged says
	// which panes they were.
	Changed() <-chan struct{}
	// TakeChanged returns the panes whose screens changed since it was last
	// called.
	TakeChanged() []domain.PaneID
	// Notices receives a signal when panes' programs have done something
	// the session should hear about, such as setting a title. Signals
	// coalesce; TakeNotices says what happened.
	Notices() <-chan struct{}
	// TakeNotices returns the notices since it was last called, oldest
	// first.
	TakeNotices() []domain.PaneNotice
}
