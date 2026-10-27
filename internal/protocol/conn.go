package protocol

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/thomassbooth/floe/internal/domain"
)

// Conn sends and receives messages over a byte stream. Send is safe to call
// from several goroutines; Receive must be called from one at a time.
type Conn struct {
	rwc io.ReadWriteCloser
	dec *gob.Decoder

	mu  sync.Mutex
	enc *gob.Encoder
}

// envelope wraps every message, so its type travels with it.
type envelope struct {
	Message Message
}

// NewConn speaks the protocol over rwc. Closing the Conn closes rwc.
func NewConn(rwc io.ReadWriteCloser) *Conn {
	return &Conn{rwc: rwc, dec: gob.NewDecoder(rwc), enc: gob.NewEncoder(rwc)}
}

// Send writes a message.
func (c *Conn) Send(m Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.enc.Encode(envelope{Message: portable(m)}); err != nil {
		return fmt.Errorf("send %T: %w", m, err)
	}
	return nil
}

// Receive reads the next message. It returns io.EOF once the other side has
// closed the connection cleanly.
func (c *Conn) Receive() (Message, error) {
	var e envelope
	if err := c.dec.Decode(&e); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("receive: %w", err)
	}
	if e.Message == nil {
		return nil, errors.New("receive: empty message")
	}
	return e.Message, nil
}

// Close closes the connection; a Receive in progress returns.
func (c *Conn) Close() error {
	return c.rwc.Close()
}

// Error is an error that crossed the connection: only its text survives.
type Error struct {
	Text string
}

func (e Error) Error() string { return e.Text }

// portable replaces errors inside events with Error, since an error's own
// type usually cannot be encoded.
func portable(m Message) Message {
	ev, ok := m.(Event)
	if !ok {
		return m
	}
	switch e := ev.Event.(type) {
	case domain.CommandFailed:
		e.Err = portableErr(e.Err)
		ev.Event = e
	case domain.Detached:
		e.Err = portableErr(e.Err)
		ev.Event = e
	}
	return ev
}

func portableErr(err error) error {
	if err == nil {
		return nil
	}
	return Error{Text: err.Error()}
}
