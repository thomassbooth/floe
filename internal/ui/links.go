package ui

import (
	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/thomassbooth/floe/internal/domain"
)

// This file is what the client does with the hyperlinks programs print. The
// outer terminal gets them along with the text, so it can underline and open
// them itself. Many terminals keep clicks for floe, which asks for the mouse,
// so a click follows a link from here too.

// linkOpenedMsg says how handing a link to the desktop went.
type linkOpenedMsg struct {
	url string
	err error
}

// uvLinks turns a grid's links into what the screen's cells hold, so that each
// is only worked out once per frame. The first entry stands for no link.
func uvLinks(links []domain.Link) []uv.Link {
	out := make([]uv.Link, len(links)+1)
	for i, l := range links {
		out[i+1].URL = l.URL
		if l.ID != "" {
			out[i+1].Params = "id=" + l.ID
		}
	}
	return out
}

// linkAt returns the link on the screen cell p of a window, if there is one.
func (m Model) linkAt(w domain.WindowView, p domain.Point) (domain.Link, bool) {
	x, y := p.X-w.Content.X, p.Y-w.Content.Y
	g, ok := m.backend.Grid(w.Pane, m.scroll[w.Pane])
	if !ok {
		return domain.Link{}, false
	}
	c := g.At(x, y)
	if c.Width == 0 && x > 0 { // the second half of a wide character
		c = g.At(x-1, y)
	}
	return g.LinkOf(c)
}

// followLink opens a link on the desktop. A file link is copied instead,
// because opening one can run it, and a link's text need not say where it
// goes. So is a link there is no way to open, such as over SSH, where a
// browser would start on the wrong machine.
func (m *Model) followLink(l domain.Link) tea.Cmd {
	if l.IsFile() || m.openLink == nil {
		return m.copyLink(l.URL)
	}
	open := m.openLink
	return func() tea.Msg { return linkOpenedMsg{url: l.URL, err: open(l.URL)} }
}

// copyLink puts a link on the clipboard, saying so in the status bar.
func (m *Model) copyLink(url string) tea.Cmd {
	m.info = "copied link " + url
	return tea.SetClipboard(url)
}

// linkOpened reports on a link handed to the desktop. One that could not be
// opened is still useful on the clipboard.
func (m *Model) linkOpened(msg linkOpenedMsg) tea.Cmd {
	if msg.err != nil {
		return m.copyLink(msg.url)
	}
	m.info = "opened " + msg.url
	return nil
}
