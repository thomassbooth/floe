package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// errEnding is returned for commands that would add to a session that is
// shutting down.
var errEnding = errors.New("session is ending")

// Run applies commands until the session ends or ctx is cancelled. It is the
// only goroutine that touches session state. The session starts its first
// window once it knows its area from SetArea.
func (s *Service) Run(ctx context.Context) error {
	defer close(s.events)
	for !s.lifecycle.done() {
		select {
		case <-ctx.Done():
			s.lifecycle.killAll()
			return ctx.Err()
		case <-s.inbox.Ready():
			for _, cmd := range s.inbox.Drain() {
				s.dispatch(ctx, cmd)
			}
		case id := <-s.host.Exited():
			s.paneExited(id)
		}
		if s.dirty && !s.lifecycle.done() {
			s.publish(ctx)
		}
	}
	err := s.lifecycle.err()
	s.emit(ctx, domain.SessionEnded{Err: err})
	return err
}

// dispatch hands a command to the service responsible for it.
func (s *Service) dispatch(ctx context.Context, cmd domain.Command) {
	if s.lifecycle.done() {
		return
	}
	var (
		changed bool
		err     error
	)
	switch c := cmd.(type) {
	case domain.SetArea:
		changed = s.layout.setArea(c.Size)
		if !s.lifecycle.started() {
			s.lifecycle.start()
			changed = true
		}
	case domain.KillSession:
		s.lifecycle.kill()
	case domain.SplitWindow:
		if s.lifecycle.ending() {
			err = errEnding
			break
		}
		err = s.windows.open(c.Dir)
		changed = err == nil
	case domain.CloseWindow:
		s.windows.closeFocused()
	case domain.FocusDirection:
		changed = s.focus.direction(c.Dir)
	case domain.FocusWindow:
		changed = s.focus.window(c.ID)
	case domain.ResizeWindow:
		changed = s.layout.resize(c.Dir, c.Amount)
	case domain.SendKey:
		s.input.sendKey(c.Key)
	case domain.Paste:
		s.input.paste(c.Text)
	default:
		err = fmt.Errorf("unsupported command %T", cmd)
	}
	s.dirty = s.dirty || changed
	if err != nil {
		s.emit(ctx, domain.CommandFailed{Command: cmd, Err: err})
	}
}

func (s *Service) paneExited(id domain.PaneID) {
	if !s.windows.paneExited(id) {
		return
	}
	s.dirty = true
	s.lifecycle.endIfEmpty()
}

// publish brings pane sizes in line with the layout and announces it.
func (s *Service) publish(ctx context.Context) {
	s.dirty = false
	s.emit(ctx, domain.LayoutChanged{Snapshot: s.layout.sync()})
}

func (s *Service) emit(ctx context.Context, ev domain.Event) {
	select {
	case s.events <- ev:
	case <-ctx.Done():
	}
}
