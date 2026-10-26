package service

import (
	"fmt"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// sessionService creates, finds, renames and ends sessions, and opens each
// session's first window once it has room for one.
type sessionService struct {
	server  *domain.Server
	host    PaneHost
	windows *windowService

	started    map[domain.SessionID]bool
	isEnding   map[domain.SessionID]bool
	anyCreated bool
}

func newSessionService(server *domain.Server, host PaneHost, windows *windowService) *sessionService {
	return &sessionService{
		server:   server,
		host:     host,
		windows:  windows,
		started:  make(map[domain.SessionID]bool),
		isEnding: make(map[domain.SessionID]bool),
	}
}

func (s *sessionService) create(name string) (*domain.Session, error) {
	sess, err := s.server.CreateSession(name)
	if err != nil {
		return nil, err
	}
	s.anyCreated = true
	return sess, nil
}

// find returns the session called name. An empty name means the most
// recently used session, created if there are none.
func (s *sessionService) find(name string) (*domain.Session, error) {
	if name == "" {
		if sess, ok := s.server.MostRecentSession(0); ok {
			return sess, nil
		}
		return s.create("")
	}
	sess, ok := s.server.SessionByName(name)
	if !ok {
		return nil, fmt.Errorf("session %q: %w", name, domain.ErrUnknownSession)
	}
	return sess, nil
}

func (s *sessionService) rename(sess *domain.Session, name string) error {
	return s.server.RenameSession(sess.ID(), name)
}

// ensureStarted opens a session's first window once the session has an
// area. It reports whether it opened one.
func (s *sessionService) ensureStarted(sess *domain.Session) (bool, error) {
	if s.started[sess.ID()] || sess.Area().Empty() {
		return false, nil
	}
	s.started[sess.ID()] = true
	if err := s.windows.open(sess, domain.Right); err != nil {
		return false, err
	}
	return true, nil
}

// kill ends every pane in a session. The session is removed once they have
// all exited; kill reports true when it already has no windows.
func (s *sessionService) kill(sess *domain.Session) bool {
	s.isEnding[sess.ID()] = true
	for _, w := range sess.Snapshot().Windows {
		s.host.Kill(w.Pane)
	}
	return sess.Len() == 0
}

func (s *sessionService) ending(sess *domain.Session) bool {
	return s.isEnding[sess.ID()]
}

// remove forgets a session whose windows have all closed.
func (s *sessionService) remove(sess *domain.Session) {
	_ = s.server.RemoveSession(sess.ID())
	delete(s.started, sess.ID())
	delete(s.isEnding, sess.ID())
}

func (s *sessionService) killAll() {
	for _, sess := range s.server.Sessions() {
		s.kill(sess)
	}
}

// created reports whether any session has ever existed.
func (s *sessionService) created() bool { return s.anyCreated }
