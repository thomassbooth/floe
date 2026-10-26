package service

import (
	"fmt"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// lifecycleService starts and ends the session: it opens the first window,
// shuts everything down on request, and records why the session ended.
type lifecycleService struct {
	session *domain.Session
	host    PaneHost
	windows *windowService

	isStarted bool
	isEnding  bool
	isDone    bool
	endErr    error
}

func newLifecycleService(session *domain.Session, host PaneHost, windows *windowService) *lifecycleService {
	return &lifecycleService{session: session, host: host, windows: windows}
}

// start opens the first window. The session ends if it cannot.
func (l *lifecycleService) start() {
	l.isStarted = true
	if err := l.windows.open(domain.Right); err != nil {
		l.end(fmt.Errorf("start session: %w", err))
	}
}

// kill ends every pane. The session ends once they have all exited.
func (l *lifecycleService) kill() {
	l.isEnding = true
	l.killAll()
	l.endIfEmpty()
}

// endIfEmpty ends the session once its last window has closed.
func (l *lifecycleService) endIfEmpty() {
	if l.isStarted && l.session.Len() == 0 {
		l.end(nil)
	}
}

func (l *lifecycleService) end(err error) {
	l.isDone = true
	l.endErr = err
	l.killAll()
}

func (l *lifecycleService) killAll() {
	for _, w := range l.session.Snapshot().Windows {
		l.host.Kill(w.Pane)
	}
}

func (l *lifecycleService) started() bool { return l.isStarted }
func (l *lifecycleService) ending() bool  { return l.isEnding }
func (l *lifecycleService) done() bool    { return l.isDone }
func (l *lifecycleService) err() error    { return l.endErr }
