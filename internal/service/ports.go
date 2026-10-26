package service

import "github.com/thomasbooth/multiplexer/internal/domain"

// PaneHost runs the processes behind panes. The infrastructure layer
// implements it.
type PaneHost interface {
	// Spawn starts a pane's process with a screen of the given size.
	Spawn(id domain.PaneID, size domain.Size) error
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
	// coalesce, so one receive may stand for many changes.
	Changed() <-chan struct{}
}
