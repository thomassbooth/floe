package service

import "github.com/thomasbooth/multiplexer/internal/domain"

// focusService decides which window has focus.
type focusService struct {
	session *domain.Session
}

func newFocusService(session *domain.Session) *focusService {
	return &focusService{session: session}
}

// direction moves focus to the neighboring window in dir. It reports whether
// focus changed.
func (f *focusService) direction(dir domain.Direction) bool {
	return f.session.FocusDirection(dir)
}

// window focuses a specific window. It reports whether focus changed.
func (f *focusService) window(id domain.WindowID) bool {
	return f.session.Focus(id)
}
