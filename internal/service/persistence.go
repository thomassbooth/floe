package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// SessionStore keeps sessions between runs. The infrastructure layer
// implements it.
type SessionStore interface {
	Load() ([]domain.SavedSession, error)
	Save([]domain.SavedSession) error
}

// saveDelay is how long changes settle before sessions are saved, so a
// burst of changes, like a window drag, is written once.
const saveDelay = time.Second

// persistenceService brings sessions back when the service starts and
// saves them as they change, so closing the terminal does not lose them.
// Saving looks up each window's directory, which can be slow, so it runs
// on its own goroutine and never holds up the actor.
type persistenceService struct {
	store    SessionStore
	host     PaneHost
	server   *domain.Server
	sessions *sessionService
	layout   *layoutService

	timer *time.Timer
	// pending hands the latest snapshot to the saver; an unsaved older one
	// is simply replaced.
	pending chan []domain.SavedSession
	saved   chan struct{} // closed when the saver has stopped

	// err is the latest error from loading or saving, for clients to see.
	// The saver sets it from its own goroutine.
	mu  sync.Mutex
	err error
}

func (p *persistenceService) fail(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

// takeErr returns the latest error, once.
func (p *persistenceService) takeErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.err
	p.err = nil
	return err
}

func newPersistenceService(store SessionStore, host PaneHost, server *domain.Server, sessions *sessionService, layout *layoutService) *persistenceService {
	p := &persistenceService{
		store:    store,
		host:     host,
		server:   server,
		sessions: sessions,
		layout:   layout,
		pending:  make(chan []domain.SavedSession, 1),
		saved:    make(chan struct{}),
	}
	if store == nil {
		close(p.saved)
		return p
	}
	go p.saver()
	return p
}

// restore brings back the saved sessions and starts their windows' shells.
// Anything that cannot be restored is skipped and reported through err.
func (p *persistenceService) restore() {
	if p.store == nil {
		return
	}
	saved, err := p.store.Load()
	if err != nil {
		p.fail(fmt.Errorf("could not restore sessions: %w", err))
		return
	}
	for _, ss := range saved {
		sess, err := p.server.RestoreSession(ss)
		if err != nil {
			p.fail(err)
			continue
		}
		dirs := make(map[domain.WindowID]string, len(ss.Windows))
		for _, sw := range ss.Windows {
			dirs[sw.ID] = sw.Dir
		}
		for _, w := range sess.Snapshot().Windows {
			if err := p.host.Spawn(w.Pane, w.Content.Size(), dirs[w.ID]); err != nil {
				_ = sess.CloseWindow(w.ID)
				p.fail(fmt.Errorf("restore session %q: %w", sess.Name(), err))
				continue
			}
			p.layout.track(w.Pane, w.Content.Size())
		}
		p.sessions.restored(sess, len(ss.Windows) > 0)
	}
}

// changed asks for a save once changes have settled.
func (p *persistenceService) changed() {
	if p.store == nil || p.timer != nil {
		return
	}
	p.timer = time.NewTimer(saveDelay)
}

// due receives when it is time to save; nil while nothing is waiting.
func (p *persistenceService) due() <-chan time.Time {
	if p.timer == nil {
		return nil
	}
	return p.timer.C
}

// save hands the current sessions to the saver.
func (p *persistenceService) save() {
	p.timer = nil
	select {
	case <-p.pending: // replace an older snapshot not yet saved
	default:
	}
	p.pending <- p.server.Save()
}

// final waits for any save in progress, then saves the current sessions
// before their shells are stopped, so their directories can still be read.
func (p *persistenceService) final() {
	if p.store == nil {
		return
	}
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	close(p.pending)
	<-p.saved
	p.write(p.server.Save())
}

func (p *persistenceService) saver() {
	defer close(p.saved)
	for snapshot := range p.pending {
		p.write(snapshot)
	}
}

// write fills in each window's directory and saves. Errors are kept for
// clients to see on the next connect; they never stop the service.
func (p *persistenceService) write(snapshot []domain.SavedSession) {
	for i := range snapshot {
		for j := range snapshot[i].Windows {
			w := &snapshot[i].Windows[j]
			w.Dir, _ = p.host.WorkingDir(w.Pane)
		}
	}
	if err := p.store.Save(snapshot); err != nil {
		p.fail(err)
	}
}
