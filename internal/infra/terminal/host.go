// Package terminal runs panes as shells in pseudo-terminals and keeps a
// virtual screen for each one. It implements service.PaneHost.
package terminal

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"

	"github.com/thomasbooth/multiplexer/internal/domain"
	"github.com/thomasbooth/multiplexer/internal/queue"
)

// killGrace is how long a pane's process gets to exit after SIGHUP before it
// is killed.
const killGrace = 2 * time.Second

// scrollbackLines is how much history each pane keeps.
const scrollbackLines = 10_000

// maxClipboard caps how much a program may copy at once, so a runaway
// program cannot flood the clipboard.
const maxClipboard = 1 << 20

// titleInterval is how often titles are checked for a new foreground
// program, which the terminal gives no notice of.
const titleInterval = 500 * time.Millisecond

// Options configures the processes a Host starts.
type Options struct {
	// Shell is the program each pane runs. It is started as a login shell.
	Shell string
	// Env is added to the inherited environment.
	Env []string
}

// Host runs one shell per pane.
type Host struct {
	opts      Options
	shellName string
	mu        sync.Mutex
	panes     map[domain.PaneID]*pane
	dirty     map[domain.PaneID]struct{}
	exited    chan domain.PaneID
	changed   chan struct{}

	// titles is the title last reported for each pane.
	titles map[domain.PaneID]string
	// notices holds what panes' programs did, until the service takes it.
	notices *queue.Unbounded[domain.PaneNotice]
	// poke wakes the title watcher early, when a program sets a title.
	poke chan struct{}
	stop chan struct{}
	once sync.Once
}

// NewHost returns a host with no panes. Call Close when done with it.
func NewHost(opts Options) *Host {
	h := &Host{
		opts:      opts,
		shellName: filepath.Base(opts.Shell),
		panes:     make(map[domain.PaneID]*pane),
		dirty:     make(map[domain.PaneID]struct{}),
		exited:    make(chan domain.PaneID, 64),
		changed:   make(chan struct{}, 1),
		titles:    make(map[domain.PaneID]string),
		notices:   queue.New[domain.PaneNotice](),
		poke:      make(chan struct{}, 1),
		stop:      make(chan struct{}),
	}
	go h.watchTitles()
	return h
}

// Close stops the host's background work and ends every pane.
func (h *Host) Close() {
	h.once.Do(func() { close(h.stop) })
	h.mu.Lock()
	panes := make([]*pane, 0, len(h.panes))
	for _, p := range h.panes {
		panes = append(panes, p)
	}
	h.mu.Unlock()
	for _, p := range panes {
		p.kill()
	}
}

// Spawn starts a shell for a pane in directory dir, or in mux's own
// directory when dir is empty or no longer exists.
func (h *Host) Spawn(id domain.PaneID, size domain.Size, dir string) error {
	size = clampSize(size)

	cmd := exec.Command(h.opts.Shell)
	cmd.Args[0] = "-" + filepath.Base(h.opts.Shell)
	cmd.Env = append(os.Environ(), h.opts.Env...)
	if info, err := os.Stat(dir); dir != "" && err == nil && info.IsDir() {
		cmd.Dir = dir
	}

	f, err := pty.StartWithSize(cmd, winsize(size))
	if err != nil {
		return fmt.Errorf("start %s: %w", h.opts.Shell, err)
	}

	p := &pane{
		cmd:        cmd,
		pty:        f,
		emu:        vt.NewEmulator(size.W, size.H),
		mouseModes: make(map[ansi.DECMode]bool),
		input:      queue.New[[]byte](),
		done:       make(chan struct{}),
	}
	p.emu.SetScrollbackSize(scrollbackLines)
	// Callbacks run inside emu.Write with p.mu held, so they only record
	// and poke; they never take the host lock.
	p.emu.SetCallbacks(vt.Callbacks{
		Title: func(t string) {
			p.oscTitle = t
			h.pokeTitles()
		},
		WorkingDirectory: func(dir string) {
			p.oscDir = dir
		},
		Bell: func() {
			h.notices.Push(domain.PaneNotice{Pane: id, Kind: domain.NoticeBell})
		},
		EnableMode: func(m ansi.Mode) {
			if d, ok := m.(ansi.DECMode); ok && isMouseMode(d) {
				p.mouseModes[d] = true
			}
		},
		DisableMode: func(m ansi.Mode) {
			if d, ok := m.(ansi.DECMode); ok {
				delete(p.mouseModes, d)
			}
		},
	})
	p.emu.RegisterOscHandler(52, func(data []byte) bool {
		if text, ok := clipboardText(data); ok {
			h.notices.Push(domain.PaneNotice{Pane: id, Kind: domain.NoticeClipboard, Text: text})
		}
		return true
	})
	h.mu.Lock()
	h.panes[id] = p
	h.titles[id] = h.shellName
	h.mu.Unlock()

	go p.readOutput(func() { h.markChanged(id) })
	go p.forwardInput()
	go p.writeInput()
	go func() {
		_ = cmd.Wait()
		p.shutdown()
		h.mu.Lock()
		delete(h.panes, id)
		delete(h.titles, id)
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

// Grid copies a pane's screen, scrolled scroll lines up into its history.
func (h *Host) Grid(id domain.PaneID, scroll int) (domain.Grid, bool) {
	p, ok := h.pane(id)
	if !ok {
		return domain.Grid{}, false
	}
	return p.grid(scroll), true
}

// State describes a pane's history and screen mode.
func (h *Host) State(id domain.PaneID) (domain.PaneState, bool) {
	p, ok := h.pane(id)
	if !ok {
		return domain.PaneState{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return domain.PaneState{
		Scrollback: p.emu.ScrollbackLen(),
		AltScreen:  p.emu.IsAltScreen(),
		Mouse:      p.mouseReport(),
	}, true
}

// SendMouse passes a mouse event to a pane's program, dropping kinds of
// event it did not ask for.
func (h *Host) SendMouse(id domain.PaneID, ev domain.MouseEvent) {
	p, ok := h.pane(id)
	if !ok {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	report := p.mouseReport()
	if report == domain.MouseReportOff {
		return
	}
	if ev.Action == domain.MouseMove {
		held := ev.Button != domain.ButtonNone
		if report == domain.MouseReportClicks || (report == domain.MouseReportDrags && !held) {
			return
		}
	}
	p.emu.SendMouse(toVTMouse(ev))
}

// WorkingDir returns the directory a pane's shell is in: the one it last
// reported, or else what the operating system says.
func (h *Host) WorkingDir(id domain.PaneID) (string, bool) {
	p, ok := h.pane(id)
	if !ok {
		return "", false
	}
	p.mu.Lock()
	dir := p.oscDir
	p.mu.Unlock()
	if dir != "" {
		return dir, true
	}
	return workingDir(p.cmd.Process.Pid)
}

// Notices implements service.PaneHost.
func (h *Host) Notices() <-chan struct{} { return h.notices.Ready() }

// TakeNotices implements service.PaneHost.
func (h *Host) TakeNotices() []domain.PaneNotice { return h.notices.Drain() }

// clipboardText decodes an OSC 52 "set clipboard" sequence, "52;c;<base64>".
// Requests to read the clipboard ("?") are refused: programs in mux must
// not see what was copied elsewhere.
func clipboardText(data []byte) (string, bool) {
	parts := bytes.SplitN(data, []byte{';'}, 3)
	if len(parts) != 3 || string(parts[2]) == "?" || base64.StdEncoding.DecodedLen(len(parts[2])) > maxClipboard {
		return "", false
	}
	text, err := base64.StdEncoding.DecodeString(string(parts[2]))
	if err != nil {
		return "", false
	}
	return string(text), true
}

func (h *Host) pokeTitles() {
	select {
	case h.poke <- struct{}{}:
	default:
	}
}

// watchTitles keeps each pane's title current, reporting changes.
func (h *Host) watchTitles() {
	tick := time.NewTicker(titleInterval)
	defer tick.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-tick.C:
		case <-h.poke:
		}
		h.refreshTitles()
	}
}

func (h *Host) refreshTitles() {
	h.mu.Lock()
	panes := make(map[domain.PaneID]*pane, len(h.panes))
	for id, p := range h.panes {
		panes[id] = p
	}
	h.mu.Unlock()

	// Work out titles without the host lock: it involves system calls.
	titles := make(map[domain.PaneID]string, len(panes))
	for id, p := range panes {
		titles[id] = p.title(h.shellName)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for id, t := range titles {
		if _, alive := h.panes[id]; alive && h.titles[id] != t {
			h.titles[id] = t
			h.notices.Push(domain.PaneNotice{Pane: id, Kind: domain.NoticeTitle, Text: t})
		}
	}
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

	mu  sync.Mutex // guards emu and the fields the emulator reports into
	emu *vt.Emulator
	// oscTitle and oscDir are the title and directory the pane's programs
	// last reported with escape sequences, if any.
	oscTitle string
	oscDir   string
	// mouseModes are the mouse reporting modes the program has turned on.
	mouseModes map[ansi.DECMode]bool

	input    *queue.Unbounded[[]byte]
	done     chan struct{}
	stopOnce sync.Once
	killOnce sync.Once
}

// title is what the pane is showing: the program in the foreground when it
// is not the shell, else the title the shell set, else the shell's name.
func (p *pane) title(shellName string) string {
	p.mu.Lock()
	osc := p.oscTitle
	p.mu.Unlock()
	if pgid, ok := foreground(p.pty); ok && pgid != p.cmd.Process.Pid {
		if name, ok := processName(pgid); ok {
			return name
		}
	}
	if osc != "" {
		return osc
	}
	return shellName
}

// mouseReport says which mouse events the program wants. p.mu must be held.
func (p *pane) mouseReport() domain.MouseReport {
	switch {
	case p.mouseModes[ansi.ModeMouseAnyEvent]:
		return domain.MouseReportMotion
	case p.mouseModes[ansi.ModeMouseButtonEvent]:
		return domain.MouseReportDrags
	case p.mouseModes[ansi.ModeMouseNormal], p.mouseModes[ansi.ModeMouseHighlight], p.mouseModes[ansi.ModeMouseX10]:
		return domain.MouseReportClicks
	}
	return domain.MouseReportOff
}

func isMouseMode(d ansi.DECMode) bool {
	switch d {
	case ansi.ModeMouseX10, ansi.ModeMouseNormal, ansi.ModeMouseHighlight,
		ansi.ModeMouseButtonEvent, ansi.ModeMouseAnyEvent:
		return true
	}
	return false
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

// grid copies the screen as seen scroll lines up into the history: the
// newest history lines fill the top rows and the screen moves down to make
// room. The cursor is only shown at the bottom, where it really is.
func (p *pane) grid(scroll int) domain.Grid {
	p.mu.Lock()
	defer p.mu.Unlock()

	w, h := p.emu.Width(), p.emu.Height()
	history := p.emu.ScrollbackLen()
	scroll = min(max(scroll, 0), history)
	g := domain.Grid{
		Size:          domain.Size{W: w, H: h},
		Cells:         make([]domain.Cell, w*h),
		CursorVisible: !p.emu.CursorHidden() && scroll == 0,
	}
	cur := p.emu.CursorPosition()
	g.Cursor = domain.Point{X: cur.X, Y: cur.Y}

	top := history - scroll // first row, counting history then screen
	for y := range h {
		line := top + y
		for x := range w {
			var c *uv.Cell
			if line < history {
				c = p.emu.ScrollbackCellAt(x, line)
			} else {
				c = p.emu.CellAt(x, line-history)
			}
			g.Cells[y*w+x] = fromVTCell(c)
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
