package service

import (
	"time"

	"github.com/thomassbooth/floe/internal/domain"
)

// layoutService moves and resizes windows and makes sure every pane's screen
// size matches the space its window gives it.
type layoutService struct {
	host PaneHost
	now  func() time.Time
	// sizes is the size each pane was last told about, and resized when.
	sizes   map[domain.PaneID]domain.Size
	resized map[domain.PaneID]time.Time
}

// redrawQuiet is how long after a resize a pane's output is taken as its
// program redrawing to fit, rather than as activity.
const redrawQuiet = 500 * time.Millisecond

func newLayoutService(host PaneHost, now func() time.Time) *layoutService {
	return &layoutService{
		host:    host,
		now:     now,
		sizes:   make(map[domain.PaneID]domain.Size),
		resized: make(map[domain.PaneID]time.Time),
	}
}

// redrawing reports whether a pane was resized so recently that its output
// is probably just its program redrawing.
func (l *layoutService) redrawing(id domain.PaneID) bool {
	at, ok := l.resized[id]
	return ok && l.now().Sub(at) < redrawQuiet
}

// setArea sets the space a session's windows are laid out in. It reports
// whether that changed.
func (l *layoutService) setArea(sess *domain.Session, size domain.Size) bool {
	area := domain.Rect{W: size.W, H: size.H}
	if sess.Area() == area {
		return false
	}
	sess.SetArea(area)
	return true
}

// resize grows or shrinks the focused window. It reports whether the layout
// changed.
func (l *layoutService) resize(sess *domain.Session, dir domain.Direction, amount int) bool {
	return sess.Resize(dir, amount)
}

// move shifts the focused window. It reports whether the layout changed.
func (l *layoutService) move(sess *domain.Session, dir domain.Direction, amount int) bool {
	return sess.Move(dir, amount)
}

// place moves and sizes a window. It reports whether the layout changed.
func (l *layoutService) place(sess *domain.Session, id domain.WindowID, frame domain.Rect) bool {
	return sess.Place(id, frame)
}

// resizeEdge moves one edge of a window. It reports whether the layout
// changed.
func (l *layoutService) resizeEdge(sess *domain.Session, id domain.WindowID, edge domain.Direction, pos int) bool {
	return sess.ResizeEdge(id, edge, pos)
}

// toggleMaximize maximizes or restores a window, or the focused one when id
// is zero. It reports whether the layout changed.
func (l *layoutService) toggleMaximize(sess *domain.Session, id domain.WindowID) bool {
	if id == 0 {
		w, ok := sess.Focused()
		if !ok {
			return false
		}
		id = w.ID
	}
	return sess.ToggleMaximize(id)
}

// snap moves a window into a zone of the screen. It reports whether the
// layout changed.
func (l *layoutService) snap(sess *domain.Session, id domain.WindowID, zone domain.Zone) bool {
	return sess.Snap(id, zone)
}

// tidy arranges every window to fill the screen. It reports whether the
// layout changed.
func (l *layoutService) tidy(sess *domain.Session) bool {
	return sess.Tidy()
}

// contentSize returns the space a window currently has for its pane.
func (l *layoutService) contentSize(sess *domain.Session, id domain.WindowID) domain.Size {
	for _, w := range sess.Snapshot().Windows {
		if w.ID == id {
			return w.Content.Size()
		}
	}
	return domain.Size{}
}

// track records the size a pane was started with.
func (l *layoutService) track(id domain.PaneID, size domain.Size) {
	l.sizes[id] = size
}

// forget drops a pane that has exited.
func (l *layoutService) forget(id domain.PaneID) {
	delete(l.sizes, id)
	delete(l.resized, id)
}

// sync resizes every pane whose window changed size and returns the
// session's layout.
func (l *layoutService) sync(sess *domain.Session) domain.Snapshot {
	snap := sess.Snapshot()
	for _, w := range snap.Windows {
		size := w.Content.Size()
		if l.sizes[w.Pane] != size {
			l.sizes[w.Pane] = size
			l.resized[w.Pane] = l.now()
			l.host.Resize(w.Pane, size)
		}
	}
	return snap
}
