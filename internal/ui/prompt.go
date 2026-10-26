package ui

import (
	"unicode"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// prompt is a one-line text input shown in the status bar.
type prompt struct {
	label  string
	text   []rune
	submit func(string) domain.Command
}

func newPrompt(label, initial string, submit func(string) domain.Command) *prompt {
	return &prompt{label: label, text: []rune(initial), submit: submit}
}

// handle reacts to a key. It returns a command to send, if any, and whether
// the prompt should close.
func (p *prompt) handle(k domain.Key) (domain.Command, bool) {
	switch {
	case k.Code == domain.KeyEnter:
		return p.submit(string(p.text)), true
	case k.Code == domain.KeyEscape:
		return nil, true
	case k.Code == domain.KeyBackspace:
		if len(p.text) > 0 {
			p.text = p.text[:len(p.text)-1]
		}
	case k.Code == 'u' && k.Mod == domain.ModCtrl:
		p.text = nil
	case k.Text != "" && k.Mod&domain.ModCtrl == 0:
		for _, r := range k.Text {
			if unicode.IsPrint(r) {
				p.text = append(p.text, r)
			}
		}
	}
	return nil, false
}
