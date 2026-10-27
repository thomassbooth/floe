package service

import "github.com/thomasbooth/multiplexer/internal/domain"

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
	// Kill ends a pane's process. Its exit is still reported on Exited.
	Kill(id domain.PaneID)
	// Grid copies a pane's current screen.
	Grid(id domain.PaneID) (domain.Grid, bool)
	// Exited receives the ID of every pane whose process ended.
	Exited() <-chan domain.PaneID
	// Changed receives a signal when any pane's screen changes. Signals
	// coalesce, so one receive may stand for many changes; TakeChanged says
	// which panes they were.
	Changed() <-chan struct{}
	// TakeChanged returns the panes whose screens changed since it was last
	// called.
	TakeChanged() []domain.PaneID
	// TitleChanged receives a signal when any pane's title changes, such
	// as when a new program starts in it. Signals coalesce.
	TitleChanged() <-chan struct{}
	// TakeTitles returns the panes whose titles changed since it was last
	// called, with their new titles.
	TakeTitles() map[domain.PaneID]string
}
