// Package protocol is how a floe client and the floe server talk: the
// messages they exchange over a connection and how those are encoded as
// bytes. It knows nothing about sockets or processes; any byte stream will
// do.
package protocol

import "github.com/thomassbooth/floe/internal/domain"

// Version is the version of the messages below. A client and server only
// talk when theirs match.
const Version = 1

// Message is anything sent over a connection.
type Message interface {
	message()
}

// Hello is the first message a client sends. Build identifies the floe
// binary, so a client never talks to a server running different code.
type Hello struct {
	Version int
	Build   string
}

// Welcome answers Hello. Err is set when the server refuses the client; it
// then closes the connection. Release is the server's version, for people
// to read.
type Welcome struct {
	Err     string
	Release string
}

// Apply carries a command from a client.
type Apply struct {
	Command domain.Command
}

// Event carries an event to a client.
type Event struct {
	Event domain.Event
}

// Changed tells a client that a pane in its session changed.
type Changed struct{}

// GridRequest asks for a pane's screen, scrolled Scroll lines into its
// history. ID matches the reply to the request.
type GridRequest struct {
	ID     uint64
	Pane   domain.PaneID
	Scroll int
}

// GridReply answers a GridRequest. OK is false when the pane is gone.
type GridReply struct {
	ID   uint64
	Grid domain.Grid
	OK   bool
}

// StateRequest asks for a pane's state.
type StateRequest struct {
	ID   uint64
	Pane domain.PaneID
}

// StateReply answers a StateRequest. OK is false when the pane is gone.
type StateReply struct {
	ID    uint64
	State domain.PaneState
	OK    bool
}

func (Hello) message()        {}
func (Welcome) message()      {}
func (Apply) message()        {}
func (Event) message()        {}
func (Changed) message()      {}
func (GridRequest) message()  {}
func (GridReply) message()    {}
func (StateRequest) message() {}
func (StateReply) message()   {}
