package ui

import "github.com/thomassbooth/floe/internal/domain"

// drag is a window move or resize in progress.
type drag struct {
	window domain.WindowID
	// move is set when dragging by the title bar; otherwise edges are being
	// dragged.
	move   bool
	offset domain.Point // mouse position within the frame while moving
	size   domain.Size  // frame size while moving
	edges  []domain.Direction
	// start is where the button went down. The drag only begins once the
	// mouse leaves that cell: some terminals report motion on the same cell
	// during a plain click, which must not move or snap anything.
	start domain.Point
	moved bool
	// zone is where the window snaps if dropped now.
	zone domain.Zone
}

// moveDrag starts moving a window grabbed by its title at p. The window
// keeps its size, snapped or not. Only a window filling the whole screen,
// which could not move otherwise, comes away smaller: at the size it had
// before being maximized, or else half the screen. The grab point keeps its
// place along the title.
func (m Model) moveDrag(w domain.WindowView, p domain.Point) *drag {
	f := w.Frame
	size := f.Size()
	if area := m.snap.Area; f == area {
		size = domain.Size{W: area.W / 2, H: area.H / 2}
		if !w.Restore.Empty() {
			size = w.Restore.Size()
		}
	}
	offset := domain.Point{X: p.X - f.X, Y: p.Y - f.Y}
	if size != f.Size() {
		offset.X = min(max(offset.X*size.W/max(f.W, 1), 1), max(size.W-2, 1))
	}
	return &drag{window: w.ID, move: true, offset: offset, size: size, start: p}
}

// resizeDrag starts resizing a window by the edges grabbed at p.
func resizeDrag(w domain.WindowView, edges []domain.Direction, p domain.Point) *drag {
	return &drag{window: w.ID, edges: edges, start: p}
}

// dragTo follows the mouse to p with absolute commands, so a fast drag that
// skips cells still lands exactly under the mouse.
func (m *Model) dragTo(p domain.Point) {
	d := m.drag
	if !d.moved && p == d.start {
		return
	}
	d.moved = true
	if d.move {
		d.zone = domain.ZoneAt(p, m.snap.Area)
		m.backend.Apply(domain.PlaceWindow{ID: d.window, Frame: domain.Rect{
			X: p.X - d.offset.X,
			Y: p.Y - d.offset.Y,
			W: d.size.W,
			H: d.size.H,
		}})
		return
	}
	for _, edge := range d.edges {
		pos := p.X
		switch edge {
		case domain.Right:
			pos = p.X + 1
		case domain.Up:
			pos = p.Y
		case domain.Down:
			pos = p.Y + 1
		}
		m.backend.Apply(domain.ResizeWindowEdge{ID: d.window, Edge: edge, Position: pos})
	}
}

// drop ends the drag at p. A moved window dropped on a snap zone snaps
// there.
func (m *Model) drop(p domain.Point) {
	d := m.drag
	m.drag = nil
	if !d.move || !d.moved {
		return
	}
	if zone := domain.ZoneAt(p, m.snap.Area); zone != domain.ZoneNone {
		m.backend.Apply(domain.SnapWindow{ID: d.window, Zone: zone})
	}
}
