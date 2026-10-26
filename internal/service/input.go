package service

import "github.com/thomasbooth/multiplexer/internal/domain"

// inputService delivers keyboard input to a session's focused pane.
type inputService struct {
	host PaneHost
}

func newInputService(host PaneHost) *inputService {
	return &inputService{host: host}
}

func (i *inputService) sendKey(sess *domain.Session, key domain.Key) {
	if w, ok := sess.Focused(); ok {
		i.host.SendKey(w.Pane, key)
	}
}

func (i *inputService) paste(sess *domain.Session, text string) {
	if w, ok := sess.Focused(); ok {
		i.host.Paste(w.Pane, text)
	}
}
