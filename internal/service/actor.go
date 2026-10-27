package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

var (
	// errEnding is returned for commands that would add to a session that
	// is shutting down.
	errEnding = errors.New("session is ending")
	// errNotAttached is returned for window commands from a client that is
	// not viewing a session.
	errNotAttached = errors.New("not attached to a session")
)

type requestKind int

const (
	requestCommand requestKind = iota
	requestConnect
	requestDisconnect
)

// request is one item in the actor's mailbox.
type request struct {
	kind   requestKind
	client domain.ClientID
	handle *Client        // requestConnect
	cmd    domain.Command // requestCommand
}

// Run serves requests until the last session ends or ctx is cancelled. It is
// the only goroutine that touches service state.
func (s *Service) Run(ctx context.Context) error {
	for !s.lifecycle.done() {
		select {
		case <-ctx.Done():
			s.sessions.killAll()
			s.clients.detachAll(nil)
			return ctx.Err()
		case <-s.inbox.Ready():
			for _, req := range s.inbox.Drain() {
				s.handle(req)
			}
		case id := <-s.host.Exited():
			s.paneExited(id)
		case <-s.host.Changed():
			s.paneOutput()
		case <-s.host.Notices():
			s.notices(s.host.TakeNotices())
		}
		s.publish()
	}
	s.clients.detachAll(nil)
	return nil
}

func (s *Service) handle(req request) {
	switch req.kind {
	case requestConnect:
		s.clients.connect(req.handle)
	case requestDisconnect:
		if old := s.clients.detach(req.client, nil); old != 0 {
			s.listChanged = true
			s.refreshArea(old)
		}
	case requestCommand:
		if !s.clients.connected(req.client) {
			return
		}
		s.server.Touch(req.client)
		if err := s.apply(req.client, req.cmd); err != nil {
			s.clients.send(req.client, domain.CommandFailed{Command: req.cmd, Err: err})
		}
		// Any command makes this client the most recently active, which
		// may hand it the session's size.
		if sess, ok := s.clients.session(req.client); ok {
			s.refreshArea(sess.ID())
		}
	}
}

// apply hands a client's command to the service responsible for it.
func (s *Service) apply(client domain.ClientID, cmd domain.Command) error {
	switch c := cmd.(type) {
	case domain.SetArea:
		s.server.SetClientSize(client, c.Size)
		if sess, ok := s.clients.session(client); ok {
			s.refreshArea(sess.ID())
		}
		return nil
	case domain.AttachSession:
		sess, err := s.sessions.find(c.Name)
		if err != nil {
			return err
		}
		return s.attach(client, sess)
	case domain.CreateSession:
		sess, err := s.sessions.create(c.Name)
		if err != nil {
			return err
		}
		return s.attach(client, sess)
	case domain.SwitchSession:
		sess, ok := s.server.Session(c.ID)
		if !ok {
			return fmt.Errorf("switch to session %d: %w", c.ID, domain.ErrUnknownSession)
		}
		return s.attach(client, sess)
	case domain.KillSession:
		id := c.ID
		if id == 0 {
			sess, ok := s.clients.session(client)
			if !ok {
				return errNotAttached
			}
			id = sess.ID()
		}
		sess, ok := s.server.Session(id)
		if !ok {
			return fmt.Errorf("kill session %d: %w", id, domain.ErrUnknownSession)
		}
		if s.sessions.kill(sess) {
			s.removeSession(sess, nil)
		}
		return nil
	}

	sess, ok := s.clients.session(client)
	if !ok {
		return errNotAttached
	}
	switch c := cmd.(type) {
	case domain.RenameSession:
		if err := s.sessions.rename(sess, c.Name); err != nil {
			return err
		}
		s.markDirty(sess.ID())
	case domain.SplitWindow:
		if s.sessions.ending(sess) {
			return errEnding
		}
		if err := s.windows.open(sess, c.Dir); err != nil {
			return err
		}
		s.markDirty(sess.ID())
	case domain.NewWindow:
		if s.sessions.ending(sess) {
			return errEnding
		}
		if err := s.windows.openAt(sess, c.Frame); err != nil {
			return err
		}
		s.markDirty(sess.ID())
	case domain.CloseWindow:
		return s.windows.close(sess, c.ID)
	case domain.FocusDirection:
		if s.focus.direction(sess, c.Dir) {
			s.markDirty(sess.ID())
		}
	case domain.FocusWindow:
		if s.focus.window(sess, c.ID) {
			s.markDirty(sess.ID())
		}
	case domain.CycleFocus:
		if s.focus.cycle(sess, c.Delta) {
			s.markDirty(sess.ID())
		}
	case domain.MoveWindow:
		if s.layout.move(sess, c.Dir, c.Amount) {
			s.markDirty(sess.ID())
		}
	case domain.ResizeWindow:
		if s.layout.resize(sess, c.Dir, c.Amount) {
			s.markDirty(sess.ID())
		}
	case domain.PlaceWindow:
		if s.layout.place(sess, c.ID, c.Frame) {
			s.markDirty(sess.ID())
		}
	case domain.ResizeWindowEdge:
		if s.layout.resizeEdge(sess, c.ID, c.Edge, c.Position) {
			s.markDirty(sess.ID())
		}
	case domain.SnapWindow:
		if s.layout.snap(sess, c.ID, c.Zone) {
			s.markDirty(sess.ID())
		}
	case domain.Tidy:
		if s.layout.tidy(sess) {
			s.markDirty(sess.ID())
		}
	case domain.ToggleMaximize:
		if s.layout.toggleMaximize(sess, c.ID) {
			s.markDirty(sess.ID())
		}
	case domain.SendKey:
		return s.input.sendKey(sess, c.Window, c.Key)
	case domain.SendMouse:
		return s.input.sendMouse(sess, c.Window, c.Event)
	case domain.Paste:
		s.input.paste(sess, c.Text)
	default:
		return fmt.Errorf("unsupported command %T", cmd)
	}
	return nil
}

// attach moves a client to a session, resizing both the session it left and
// the one it joined.
func (s *Service) attach(client domain.ClientID, sess *domain.Session) error {
	old, err := s.clients.attach(client, sess.ID())
	if err != nil {
		return err
	}
	if old != 0 && old != sess.ID() {
		s.refreshArea(old)
	}
	s.listChanged = true
	s.markDirty(sess.ID())
	s.refreshArea(sess.ID())
	return nil
}

// refreshArea lays a session out in the space its clients give it, and
// starts its first window once it has space.
func (s *Service) refreshArea(id domain.SessionID) {
	sess, ok := s.server.Session(id)
	if !ok {
		return
	}
	if size, ok := s.server.AreaFor(id); ok && s.layout.setArea(sess, size) {
		s.markDirty(id)
	}
	opened, err := s.sessions.ensureStarted(sess)
	switch {
	case err != nil:
		s.removeSession(sess, fmt.Errorf("start session %q: %w", sess.Name(), err))
	case opened:
		s.markDirty(id)
	}
}

func (s *Service) paneExited(id domain.PaneID) {
	sess, closed := s.windows.paneExited(id)
	if !closed {
		return
	}
	s.markDirty(sess.ID())
	if sess.Len() == 0 {
		s.removeSession(sess, nil)
	}
}

// removeSession deletes a session and moves its clients to the most
// recently used remaining session, detaching them if there is none.
func (s *Service) removeSession(sess *domain.Session, cause error) {
	clients := s.server.ClientsOf(sess.ID())
	s.sessions.remove(sess)
	delete(s.dirty, sess.ID())
	s.listChanged = true

	for _, c := range clients {
		next, ok := s.server.MostRecentSession(0)
		if !ok {
			s.clients.detach(c, cause)
			continue
		}
		if cause != nil {
			s.clients.send(c, domain.CommandFailed{Err: cause})
		}
		_ = s.attach(c, next)
	}
	s.lifecycle.check()
}

// notices handles what panes' programs reported.
func (s *Service) notices(notices []domain.PaneNotice) {
	for _, n := range notices {
		sess, ok := s.server.SessionForPane(n.Pane)
		if !ok {
			continue
		}
		switch n.Kind {
		case domain.NoticeTitle:
			if s.windows.retitle(sess, n.Pane, n.Text) {
				s.markDirty(sess.ID())
			}
		case domain.NoticeClipboard:
			for _, c := range s.server.ClientsOf(sess.ID()) {
				s.clients.send(c, domain.ClipboardSet{Text: n.Text})
			}
		}
	}
}

// paneOutput tells the clients viewing changed panes to redraw.
func (s *Service) paneOutput() {
	sessions := map[domain.SessionID]bool{}
	for _, p := range s.host.TakeChanged() {
		if sess, ok := s.server.SessionForPane(p); ok {
			sessions[sess.ID()] = true
		}
	}
	for id := range sessions {
		for _, c := range s.server.ClientsOf(id) {
			s.clients.signalChanged(c)
		}
	}
}

func (s *Service) markDirty(id domain.SessionID) {
	s.dirty[id] = true
}

// publish brings pane sizes in line with each changed layout and tells the
// clients about it.
func (s *Service) publish() {
	if len(s.dirty) == 0 && !s.listChanged {
		return
	}
	// The session list goes first so clients never draw a layout next to a
	// stale list.
	s.clients.broadcastSessions()
	for id := range s.dirty {
		sess, ok := s.server.Session(id)
		if !ok {
			continue
		}
		ev := domain.LayoutChanged{Snapshot: s.layout.sync(sess)}
		for _, c := range s.server.ClientsOf(id) {
			s.clients.send(c, ev)
		}
	}
	clear(s.dirty)
	s.listChanged = false
}
