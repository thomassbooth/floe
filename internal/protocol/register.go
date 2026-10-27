package protocol

import (
	"encoding/gob"

	"github.com/thomassbooth/floe/internal/domain"
)

// Every concrete type that travels inside an interface must be registered
// with gob. The tests check these lists against the domain package.
var (
	messages = []Message{
		Hello{}, Welcome{}, Apply{}, Event{}, Changed{},
		GridRequest{}, GridReply{}, StateRequest{}, StateReply{},
	}
	commands = []domain.Command{
		domain.SetArea{}, domain.AttachSession{}, domain.CreateSession{},
		domain.SwitchSession{}, domain.RenameSession{}, domain.KillSession{},
		domain.NewWindow{}, domain.SplitWindow{}, domain.FocusDirection{},
		domain.FocusWindow{}, domain.ResizeWindow{}, domain.MoveWindow{},
		domain.CycleFocus{}, domain.PlaceWindow{}, domain.ResizeWindowEdge{},
		domain.ToggleMaximize{}, domain.SnapWindow{}, domain.Tidy{},
		domain.CloseWindow{}, domain.SendKey{}, domain.SendMouse{},
		domain.Paste{},
	}
	events = []domain.Event{
		domain.LayoutChanged{}, domain.SessionsChanged{}, domain.ClipboardSet{},
		domain.CommandFailed{}, domain.Detached{},
	}
)

func init() {
	for _, m := range messages {
		gob.Register(m)
	}
	for _, c := range commands {
		gob.Register(c)
	}
	for _, e := range events {
		gob.Register(e)
	}
	gob.Register(Error{})
}
