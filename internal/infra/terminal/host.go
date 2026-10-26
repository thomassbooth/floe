// Package terminal runs panes as shells in pseudo-terminals and keeps a
// virtual screen for each one. It implements service.PaneHost.
package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"

	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/queue"
)

// killGrace is how long a pane's process gets to exit after SIGHUP before it
// is killed.
const killGrace = 2 * time.Second

// Options configures the processes a Host starts.
type Options struct {
	// Shell is the program each pane runs. It is started as a login shell.
	Shell string
	// Env is added to the inherited environment.
	Env []string
}

// Host runs one shell per pane.
type Host struct {
	opts    Options
	mu      sync.Mutex
	panes   map[domain.PaneID]*pane
	dirty   map[domain.PaneID]struct{}
	exited  chan domain.PaneID
	changed chan struct{}
}

// NewHost returns a host with no panes.
func NewHost(opts Options) *Host {
	return &Host{
		opts:    opts,
		panes:   make(map[domain.PaneID]*pane),
		dirty:   make(map[domain.PaneID]struct{}),
		exited:  make(chan domain.PaneID, 64),
		changed: make(chan struct{}, 1),
	}
}

// Spawn starts a shell for a pane.
func (h *Host) Spawn(id domain.PaneID, size domain.Size) error {
	size = clampSize(size)

	cmd := exec.Command(h.opts.Shell)
	cmd.Args[0] = "-" + filepath.Base(h.opts.Shell)
	cmd.Env = append(os.Environ(), h.opts.Env...)

	f, err := pty.StartWithSize(cmd, winsize(size))
	if err != nil {
		return fmt.Errorf("start %s: %w", h.opts.Shell, err)
	}

	p := &pane{
		cmd:   cmd,
		pty:   f,
		emu:   vt.NewEmulator(size.W, size.H),
		input: queue.New[[]byte](),
		done:  make(chan struct{}),
	}
	h.mu.Lock()
	h.panes[id] = p
	h.mu.Unlock()

	go p.readOutput(func() { h.markChanged(id) })
	go p.forwardInput()
	go p.writeInput()
	go func() {
		_ = cmd.Wait()
		p.shutdown()
		h.mu.Lock()
		delete(h.panes, id)
		h.mu.Unlock()
		h.exited <- id
	}()
	return nil
}

// Resize changes a pane's screen size.
func (h *Host) Resize(id domain.PaneID, size domain.Size) {
	if p, ok := h.pane(id); ok {
		p.resize(clampSize(size))
	}
}

// SendKey types a key into a pane.
func (h *Host) SendKey(id domain.PaneID, key domain.Key) {
	if p, ok := h.pane(id); ok {
		p.mu.Lock()
		p.emu.SendKey(toVTKey(key))
		p.mu.Unlock()
	}
}

// Paste sends pasted text to a pane, bracketed if the program asked for it.
func (h *Host) Paste(id domain.PaneID, text string) {
	if p, ok := h.pane(id); ok {
		p.mu.Lock()
		p.emu.Paste(text)
		p.mu.Unlock()
	}
}

// Kill hangs up a pane's shell, killing it if it does not exit in time.
func (h *Host) Kill(id domain.PaneID) {
	if p, ok := h.pane(id); ok {
		p.kill()
	}
}

// Grid copies a pane's current screen.
func (h *Host) Grid(id domain.PaneID) (domain.Grid, bool) {
	p, ok := h.pane(id)
	if !ok {
		return domain.Grid{}, false
	}
	return p.grid(), true
}

// Exited implements service.PaneHost.
func (h *Host) Exited() <-chan domain.PaneID { return h.exited }

// Changed implements service.PaneHost.
func (h *Host) Changed() <-chan struct{} { return h.changed }

// TakeChanged implements service.PaneHost.
func (h *Host) TakeChanged() []domain.PaneID {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]domain.PaneID, 0, len(h.dirty))
	for id := range h.dirty {
		out = append(out, id)
	}
	clear(h.dirty)
	return out
}

func (h *Host) pane(id domain.PaneID) (*pane, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.panes[id]
	return p, ok
}

func (h *Host) markChanged(id domain.PaneID) {
	h.mu.Lock()
	h.dirty[id] = struct{}{}
	h.mu.Unlock()
	select {
	case h.changed <- struct{}{}:
	default:
	}
}

// pane is one running shell and its virtual screen.
//
// Data flows through three goroutines so that no lock is ever held while
// waiting on the shell:
//
//	readOutput:   pty  -> emulator
//	forwardInput: emulator input (keys, query replies) -> queue
//	writeInput:   queue -> pty
type pane struct {
	cmd *exec.Cmd
	pty *os.File

	mu  sync.Mutex // guards emu
	emu *vt.Emulator

	input    *queue.Unbounded[[]byte]
	done     chan struct{}
	stopOnce sync.Once
	killOnce sync.Once
}

func (p *pane) readOutput(changed func()) {
	buf := make([]byte, 32*1024)
	for {
		n, err := p.pty.Read(buf)
		if n > 0 {
			p.mu.Lock()
			_, _ = p.emu.Write(buf[:n])
			p.mu.Unlock()
			changed()
		}
		if err != nil {
			return
		}
	}
}

func (p *pane) forwardInput() {
	buf := make([]byte, 4096)
	for {
		n, err := p.emu.Read(buf)
		if n > 0 {
			p.input.Push(append([]byte(nil), buf[:n]...))
		}
		if err != nil {
			return
		}
	}
}

func (p *pane) writeInput() {
	for {
		select {
		case <-p.done:
			return
		case <-p.input.Ready():
			for _, b := range p.input.Drain() {
				if _, err := p.pty.Write(b); err != nil {
					return
				}
			}
		}
	}
}

func (p *pane) resize(size domain.Size) {
	p.mu.Lock()
	p.emu.Resize(size.W, size.H)
	p.mu.Unlock()
	_ = pty.Setsize(p.pty, winsize(size))
}

func (p *pane) grid() domain.Grid {
	p.mu.Lock()
	defer p.mu.Unlock()

	w, h := p.emu.Width(), p.emu.Height()
	g := domain.Grid{
		Size:          domain.Size{W: w, H: h},
		Cells:         make([]domain.Cell, w*h),
		CursorVisible: !p.emu.CursorHidden(),
	}
	cur := p.emu.CursorPosition()
	g.Cursor = domain.Point{X: cur.X, Y: cur.Y}
	for y := range h {
		for x := range w {
			g.Cells[y*w+x] = fromVTCell(p.emu.CellAt(x, y))
		}
	}
	return g
}

func (p *pane) kill() {
	p.killOnce.Do(func() {
		pid := p.cmd.Process.Pid
		// The shell leads its own process group, so this reaches its
		// foreground jobs too.
		_ = syscall.Kill(-pid, syscall.SIGHUP)
		time.AfterFunc(killGrace, func() {
			select {
			case <-p.done:
			default:
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		})
	})
}

// shutdown releases the pane's resources once its process has exited.
func (p *pane) shutdown() {
	p.stopOnce.Do(func() {
		close(p.done)
		_ = p.pty.Close()
		// Ends forwardInput. Closing the pipe rather than the emulator
		// avoids racing with its unsynchronized closed flag.
		if c, ok := p.emu.InputPipe().(io.Closer); ok {
			_ = c.Close()
		}
	})
}

func clampSize(s domain.Size) domain.Size {
	return domain.Size{W: max(s.W, 1), H: max(s.H, 1)}
}

func winsize(s domain.Size) *pty.Winsize {
	return &pty.Winsize{Cols: uint16(s.W), Rows: uint16(s.H)}
}

// ErrNoShell is returned by DefaultShell when no shell can be found.
var ErrNoShell = errors.New("no shell found")

// DefaultShell returns the user's shell from $SHELL, falling back to
// /bin/sh.
func DefaultShell() (string, error) {
	for _, s := range []string{os.Getenv("SHELL"), "/bin/sh"} {
		if s == "" {
			continue
		}
		if _, err := os.Stat(s); err == nil {
			return s, nil
		}
	}
	return "", ErrNoShell
}
