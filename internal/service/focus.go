package service

import "github.com/thomasbooth/multiplexer/internal/domain"

// focusService decides which window in a session has focus.
type focusService struct{}

func newFocusService() *focusService {
	return &focusService{}
}

// direction moves focus to the neighboring window in dir. It reports whether
// focus changed.
func (f *focusService) direction(sess *domain.Session, dir domain.Direction) bool {
	return sess.FocusDirection(dir)
}

// window focuses a specific window. It reports whether focus changed.
func (f *focusService) window(sess *domain.Session, id domain.WindowID) bool {
	return sess.Focus(id)
}
