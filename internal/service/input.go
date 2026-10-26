package service

import "github.com/thomasbooth/multiplexer/internal/domain"

// inputService delivers keyboard input to the focused pane.
type inputService struct {
	session *domain.Session
	host    PaneHost
}

func newInputService(session *domain.Session, host PaneHost) *inputService {
	return &inputService{session: session, host: host}
}

func (i *inputService) sendKey(key domain.Key) {
	if w, ok := i.session.Focused(); ok {
		i.host.SendKey(w.Pane, key)
	}
}

func (i *inputService) paste(text string) {
	if w, ok := i.session.Focused(); ok {
		i.host.Paste(w.Pane, text)
	}
}
