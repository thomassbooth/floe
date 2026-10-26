package service

import "github.com/thomasbooth/multiplexer/internal/domain"

// clientService tracks connected clients: which session each one views and
// how to reach it.
type clientService struct {
	server  *domain.Server
	handles map[domain.ClientID]*Client
}

func newClientService(server *domain.Server) *clientService {
	return &clientService{server: server, handles: make(map[domain.ClientID]*Client)}
}

func (c *clientService) connect(h *Client) {
	c.server.AddClient(h.id)
	c.handles[h.id] = h
}

func (c *clientService) connected(id domain.ClientID) bool {
	_, ok := c.handles[id]
	return ok
}

// session returns the session a client is viewing.
func (c *clientService) session(id domain.ClientID) (*domain.Session, bool) {
	client, ok := c.server.Client(id)
	if !ok || client.Session == 0 {
		return nil, false
	}
	return c.server.Session(client.Session)
}

// attach points a client at a session and returns the one it left.
func (c *clientService) attach(id domain.ClientID, session domain.SessionID) (domain.SessionID, error) {
	client, _ := c.server.Client(id)
	if err := c.server.Attach(id, session); err != nil {
		return 0, err
	}
	return client.Session, nil
}

// detach sends a client its final event and forgets it. It returns the
// session the client was viewing.
func (c *clientService) detach(id domain.ClientID, cause error) domain.SessionID {
	h, ok := c.handles[id]
	if !ok {
		return 0
	}
	client, _ := c.server.Client(id)
	h.deliver(domain.Detached{Err: cause})
	c.server.RemoveClient(id)
	delete(c.handles, id)
	return client.Session
}

func (c *clientService) detachAll(cause error) {
	for id := range c.handles {
		c.detach(id, cause)
	}
}

func (c *clientService) send(id domain.ClientID, ev domain.Event) {
	if h, ok := c.handles[id]; ok {
		h.deliver(ev)
	}
}

func (c *clientService) signalChanged(id domain.ClientID) {
	if h, ok := c.handles[id]; ok {
		h.signalChanged()
	}
}

// broadcastSessions sends every client the current session list.
func (c *clientService) broadcastSessions() {
	infos := c.server.Infos()
	for id, h := range c.handles {
		client, _ := c.server.Client(id)
		h.deliver(domain.SessionsChanged{Sessions: infos, Current: client.Session})
	}
}
