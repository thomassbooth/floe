package domain

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// SessionID identifies a session. Zero means no session.
type SessionID int

// ClientID identifies a connected client, such as a terminal attached to a
// session. Zero means no client.
type ClientID int

var (
	// ErrUnknownSession is returned when a session does not exist.
	ErrUnknownSession = errors.New("unknown session")
	// ErrSessionExists is returned when a session name is already taken.
	ErrSessionExists = errors.New("session already exists")
	// ErrInvalidName is returned for empty or unprintable session names.
	ErrInvalidName = errors.New("invalid session name")
	// ErrUnknownClient is returned when a client is not connected.
	ErrUnknownClient = errors.New("unknown client")
)

// maxNameLen keeps session names short enough for the status bar.
const maxNameLen = 32

// Client is a connection viewing at most one session.
type Client struct {
	ID      ClientID
	Session SessionID
	// Size is the area the client can show windows in.
	Size Size
	// active is the logical time of the client's last activity.
	active uint64
}

// Server is every session and every client connected to them. It decides
// session names, which session a client sees, and how big each session's
// area is when several clients share it.
type Server struct {
	sessions map[SessionID]*Session
	clients  map[ClientID]*Client
	// used is the logical time each session was last attached to or used.
	used map[SessionID]uint64

	clock       uint64
	lastSession SessionID
	lastPane    PaneID
}

// NewServer returns a server with no sessions or clients.
func NewServer() *Server {
	return &Server{
		sessions: make(map[SessionID]*Session),
		clients:  make(map[ClientID]*Client),
		used:     make(map[SessionID]uint64),
	}
}

// CreateSession adds a session. An empty name picks the lowest unused
// number, like tmux.
func (s *Server) CreateSession(name string) (*Session, error) {
	if name == "" {
		name = s.freeName()
	}
	name, err := s.checkName(name, 0)
	if err != nil {
		return nil, err
	}
	s.lastSession++
	sess := NewSession(s.lastSession, name)
	s.sessions[sess.ID()] = sess
	s.used[sess.ID()] = s.tick()
	return sess, nil
}

// RenameSession changes a session's name.
func (s *Server) RenameSession(id SessionID, name string) error {
	sess, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("rename session %d: %w", id, ErrUnknownSession)
	}
	name, err := s.checkName(name, id)
	if err != nil {
		return err
	}
	sess.name = name
	return nil
}

// RemoveSession deletes a session. Clients attached to it become detached.
func (s *Server) RemoveSession(id SessionID) error {
	if _, ok := s.sessions[id]; !ok {
		return fmt.Errorf("remove session %d: %w", id, ErrUnknownSession)
	}
	delete(s.sessions, id)
	delete(s.used, id)
	for _, c := range s.clients {
		if c.Session == id {
			c.Session = 0
		}
	}
	return nil
}

// Session returns a session by ID.
func (s *Server) Session(id SessionID) (*Session, bool) {
	sess, ok := s.sessions[id]
	return sess, ok
}

// SessionByName returns a session by name.
func (s *Server) SessionByName(name string) (*Session, bool) {
	for _, sess := range s.sessions {
		if sess.Name() == name {
			return sess, true
		}
	}
	return nil, false
}

// Sessions returns every session, oldest first.
func (s *Server) Sessions() []*Session {
	out := slices.Collect(maps.Values(s.sessions))
	slices.SortFunc(out, func(a, b *Session) int { return cmp.Compare(a.ID(), b.ID()) })
	return out
}

// Len returns the number of sessions.
func (s *Server) Len() int { return len(s.sessions) }

// MostRecentSession returns the session used most recently, skipping
// exclude.
func (s *Server) MostRecentSession(exclude SessionID) (*Session, bool) {
	var best *Session
	for id, sess := range s.sessions {
		if id == exclude {
			continue
		}
		if best == nil || s.used[id] > s.used[best.ID()] {
			best = sess
		}
	}
	return best, best != nil
}

// SessionForPane returns the session showing a pane.
func (s *Server) SessionForPane(p PaneID) (*Session, bool) {
	for _, sess := range s.sessions {
		if _, ok := sess.WindowForPane(p); ok {
			return sess, true
		}
	}
	return nil, false
}

// NewPaneID allocates a pane ID that is unique across all sessions.
func (s *Server) NewPaneID() PaneID {
	s.lastPane++
	return s.lastPane
}

// Infos describes every session, oldest first.
func (s *Server) Infos() []SessionInfo {
	sessions := s.Sessions()
	out := make([]SessionInfo, len(sessions))
	for i, sess := range sessions {
		out[i] = SessionInfo{
			ID:      sess.ID(),
			Name:    sess.Name(),
			Windows: sess.Len(),
			Clients: len(s.ClientsOf(sess.ID())),
		}
	}
	return out
}

// AddClient registers a client that is not yet attached.
func (s *Server) AddClient(id ClientID) {
	s.clients[id] = &Client{ID: id, active: s.tick()}
}

// RemoveClient forgets a client.
func (s *Server) RemoveClient(id ClientID) {
	delete(s.clients, id)
}

// Client returns a connected client.
func (s *Server) Client(id ClientID) (Client, bool) {
	c, ok := s.clients[id]
	if !ok {
		return Client{}, false
	}
	return *c, true
}

// Attach points a client at a session.
func (s *Server) Attach(client ClientID, session SessionID) error {
	c, ok := s.clients[client]
	if !ok {
		return fmt.Errorf("attach client %d: %w", client, ErrUnknownClient)
	}
	if _, ok := s.sessions[session]; !ok {
		return fmt.Errorf("attach to session %d: %w", session, ErrUnknownSession)
	}
	c.Session = session
	s.Touch(client)
	return nil
}

// SetClientSize records how much space a client has for windows.
func (s *Server) SetClientSize(client ClientID, size Size) {
	if c, ok := s.clients[client]; ok {
		c.Size = size
	}
}

// Touch marks a client, and the session it views, as just used.
func (s *Server) Touch(client ClientID) {
	c, ok := s.clients[client]
	if !ok {
		return
	}
	now := s.tick()
	c.active = now
	if c.Session != 0 {
		s.used[c.Session] = now
	}
}

// ClientsOf returns the clients attached to a session, in ID order.
func (s *Server) ClientsOf(session SessionID) []ClientID {
	var out []ClientID
	for id, c := range s.clients {
		if c.Session == session {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// AreaFor returns the size a session should be laid out in: that of the
// most recently active client attached to it. It reports false when no
// client with a known size is attached.
func (s *Server) AreaFor(session SessionID) (Size, bool) {
	var best *Client
	for _, c := range s.clients {
		if c.Session != session || c.Size == (Size{}) {
			continue
		}
		if best == nil || c.active > best.active {
			best = c
		}
	}
	if best == nil {
		return Size{}, false
	}
	return best.Size, true
}

func (s *Server) tick() uint64 {
	s.clock++
	return s.clock
}

// checkName validates a name for session self, which may keep its own name.
func (s *Server) checkName(name string, self SessionID) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > maxNameLen || strings.ContainsFunc(name, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return "", fmt.Errorf("%q: %w", name, ErrInvalidName)
	}
	if other, ok := s.SessionByName(name); ok && other.ID() != self {
		return "", fmt.Errorf("%q: %w", name, ErrSessionExists)
	}
	return name, nil
}

func (s *Server) freeName() string {
	for n := 1; ; n++ {
		name := strconv.Itoa(n)
		if _, taken := s.SessionByName(name); !taken {
			return name
		}
	}
}

// SessionInfo summarizes a session for listings such as a switcher.
type SessionInfo struct {
	ID      SessionID
	Name    string
	Windows int
	Clients int
}
