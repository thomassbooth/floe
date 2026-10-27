package service

import "github.com/thomassbooth/floe/internal/domain"

// lifecycleService decides when the service stops: once sessions have
// existed and the last one has ended, like the tmux server.
type lifecycleService struct {
	server   *domain.Server
	sessions *sessionService
	isDone   bool
}

func newLifecycleService(server *domain.Server, sessions *sessionService) *lifecycleService {
	return &lifecycleService{server: server, sessions: sessions}
}

// check stops the service if no sessions remain.
func (l *lifecycleService) check() {
	if l.sessions.created() && l.server.Len() == 0 {
		l.isDone = true
	}
}

func (l *lifecycleService) done() bool { return l.isDone }
