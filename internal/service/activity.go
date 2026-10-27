package service

import (
	"time"

	"github.com/thomassbooth/floe/internal/domain"
)

// activityService decides which panes are busy: their program keeps
// printing, rather than drawing a prompt once, echoing what was typed or
// redrawing after a resize.
type activityService struct {
	server *domain.Server
	layout *layoutService
	now    func() time.Time
	after  func(time.Duration) <-chan time.Time
	// output is each printing pane's current run of output, and input when
	// each pane was last sent a key, a paste or a mouse event.
	output map[domain.PaneID]run
	input  map[domain.PaneID]time.Time
	// timer fires when the busy pane that printed longest ago may have gone
	// quiet; nil while no pane is busy.
	timer <-chan time.Time
}

// run is a stretch of output with no gap as long as busyQuiet.
type run struct {
	first, last time.Time
	busy        bool
}

// busyAfter is how long a pane must keep printing to be busy, so that a
// single burst, such as a prompt or a command's result, is not.
const busyAfter = 500 * time.Millisecond

// busyQuiet is how long a pane must stay silent before it stops being busy.
// It bridges the pauses between a working program's updates, such as a
// spinner's frames or a build's steps.
const busyQuiet = 1500 * time.Millisecond

// echoQuiet is how long after input an idle pane's output is taken as the
// echo of that input, rather than as its program starting to work.
const echoQuiet = 500 * time.Millisecond

func newActivityService(server *domain.Server, layout *layoutService, now func() time.Time, after func(time.Duration) <-chan time.Time) *activityService {
	return &activityService{
		server: server,
		layout: layout,
		now:    now,
		after:  after,
		output: make(map[domain.PaneID]run),
		input:  make(map[domain.PaneID]time.Time),
	}
}

// typed notes that a pane was sent input, so its echo is not taken as work.
func (a *activityService) typed(id domain.PaneID) {
	a.input[id] = a.now()
}

// printed notes that a pane in sess changed its screen, and marks it busy
// once it has kept printing for busyAfter. It reports whether the pane's
// window changed.
func (a *activityService) printed(sess *domain.Session, id domain.PaneID) bool {
	now := a.now()
	r, ok := a.output[id]
	if ok && now.Sub(r.last) >= busyQuiet {
		ok = false // an earlier run that never became busy
	}
	if !ok {
		if a.layout.redrawing(id) || now.Sub(a.input[id]) < echoQuiet {
			return false
		}
		r = run{first: now}
	}
	// A busy pane stays busy for as long as it prints anything.
	r.last = now
	changed := false
	if !r.busy && now.Sub(r.first) >= busyAfter {
		r.busy = true
		changed = sess.SetBusy(id, true)
		if a.timer == nil {
			a.timer = a.after(busyQuiet)
		}
	}
	a.output[id] = r
	return changed
}

// due receives when a busy pane may have gone quiet; nil while none is busy.
func (a *activityService) due() <-chan time.Time {
	return a.timer
}

// settle clears panes that have been quiet long enough and returns the
// sessions whose windows changed.
func (a *activityService) settle() []*domain.Session {
	a.timer = nil
	now := a.now()
	var changed []*domain.Session
	next := time.Duration(-1)
	for id, r := range a.output {
		if !r.busy {
			continue
		}
		if left := busyQuiet - now.Sub(r.last); left > 0 {
			if next < 0 || left < next {
				next = left
			}
			continue
		}
		delete(a.output, id)
		if sess, ok := a.server.SessionForPane(id); ok && sess.SetBusy(id, false) {
			changed = append(changed, sess)
		}
	}
	if next >= 0 {
		a.timer = a.after(next)
	}
	return changed
}

// forget drops a pane that has exited.
func (a *activityService) forget(id domain.PaneID) {
	delete(a.output, id)
	delete(a.input, id)
}
