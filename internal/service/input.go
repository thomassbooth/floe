package service

import (
	"fmt"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// inputService delivers keyboard and mouse input to a session's panes.
type inputService struct {
	host PaneHost
}

func newInputService(host PaneHost) *inputService {
	return &inputService{host: host}
}

// sendKey types a key into a window's pane, the focused window's when id is
// zero.
func (i *inputService) sendKey(sess *domain.Session, id domain.WindowID, key domain.Key) error {
	if id == 0 {
		if w, ok := sess.Focused(); ok {
			i.host.SendKey(w.Pane, key)
		}
		return nil
	}
	for _, w := range sess.Snapshot().Windows {
		if w.ID == id {
			i.host.SendKey(w.Pane, key)
			return nil
		}
	}
	return fmt.Errorf("send key to window %d: %w", id, domain.ErrUnknownWindow)
}

// sendMouse passes a mouse event to a window's program.
func (i *inputService) sendMouse(sess *domain.Session, id domain.WindowID, ev domain.MouseEvent) error {
	for _, w := range sess.Snapshot().Windows {
		if w.ID == id {
			i.host.SendMouse(w.Pane, ev)
			return nil
		}
	}
	return fmt.Errorf("send mouse to window %d: %w", id, domain.ErrUnknownWindow)
}

func (i *inputService) paste(sess *domain.Session, text string) {
	if w, ok := sess.Focused(); ok {
		i.host.Paste(w.Pane, text)
	}
}
