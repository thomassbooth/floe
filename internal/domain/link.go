package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxLinkURL and maxLinkID cap how long a link a program can have floe carry
// and pass on to the outer terminal.
const (
	maxLinkURL = 2048
	maxLinkID  = 256
)

// Link is a hyperlink a program printed around some text, with OSC 8.
type Link struct {
	URL string
	// ID ties together cells of one link that are not next to each other,
	// such as one wrapped over two lines. It is often empty.
	ID string
}

// NewLink makes a link from what a program sent: a URL and OSC 8 parameters,
// written "id=x:other=y". It reports false for a link that must not reach the
// outer terminal: one with a scheme other than http, https, file or mailto,
// one that is too long, or one holding control characters. Only the id
// parameter is kept, and only when it is safe to pass on.
func NewLink(url, params string) (Link, bool) {
	scheme, rest, ok := strings.Cut(url, ":")
	if !ok || rest == "" || len(url) > maxLinkURL || !printable(url) {
		return Link{}, false
	}
	switch strings.ToLower(scheme) {
	case "http", "https", "file", "mailto":
	default:
		return Link{}, false
	}
	link := Link{URL: url}
	for param := range strings.SplitSeq(params, ":") {
		if id, ok := strings.CutPrefix(param, "id="); ok && len(id) <= maxLinkID && printable(id) && !strings.Contains(id, ";") {
			link.ID = id
		}
	}
	return link, true
}

// IsFile reports whether a link points at a local file. Opening one can run
// it, so it takes more care than a web page.
func (l Link) IsFile() bool {
	scheme, _, _ := strings.Cut(l.URL, ":")
	return strings.EqualFold(scheme, "file")
}

// printable reports whether s is valid UTF-8 without control characters,
// which is what keeps it from ending an escape sequence early.
func printable(s string) bool {
	return utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}
