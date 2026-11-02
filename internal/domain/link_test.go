package domain

import (
	"strings"
	"testing"
)

func TestNewLinkLetsSafeLinksThrough(t *testing.T) {
	for _, url := range []string{
		"http://example.com",
		"https://example.com/a?b=c#d",
		"HTTPS://EXAMPLE.COM",
		"file:///tmp/a.txt",
		"file://host/tmp/a.txt",
		"mailto:someone@example.com",
		"https://example.com/é",
	} {
		if link, ok := NewLink(url, ""); !ok || link.URL != url {
			t.Errorf("NewLink(%q) = %+v, %v", url, link, ok)
		}
	}
}

func TestNewLinkRefusesUnsafeLinks(t *testing.T) {
	for name, url := range map[string]string{
		"javascript":  "javascript:alert(1)",
		"data":        "data:text/html,x",
		"custom":      "x-man-page://ls",
		"no scheme":   "example.com",
		"empty":       "",
		"only scheme": "https:",
		"escape":      "https://example.com/\x1b]2;owned\x07",
		"newline":     "https://example.com/\nx",
		"c1 control":  "https://example.com/\u009b31m",
		"bad utf-8":   "https://example.com/\xff",
		"too long":    "https://example.com/" + strings.Repeat("a", maxLinkURL),
	} {
		if link, ok := NewLink(url, ""); ok {
			t.Errorf("%s: NewLink(%q) = %+v, want it refused", name, url, link)
		}
	}
}

func TestNewLinkKeepsOnlyASafeID(t *testing.T) {
	for params, want := range map[string]string{
		"":                      "",
		"id=abc":                "abc",
		"other=1:id=abc":        "abc",
		"id=abc:other=1":        "abc",
		"other=1":               "",
		"id=a\x1bb":             "",
		"id=a;b":                "",
		"id=" + long(300):       "",
		"id=" + long(maxLinkID): long(maxLinkID),
	} {
		link, ok := NewLink("https://example.com", params)
		if !ok || link.ID != want {
			t.Errorf("params %q: got %+v, %v, want id %q", params, link, ok, want)
		}
	}
}

func long(n int) string { return strings.Repeat("a", n) }

func TestLinkIsFile(t *testing.T) {
	for url, want := range map[string]bool{
		"file:///tmp/a":       true,
		"FILE:///tmp/a":       true,
		"https://example.com": false,
		"mailto:a@b.c":        false,
	} {
		if got := (Link{URL: url}).IsFile(); got != want {
			t.Errorf("%q: IsFile = %v", url, got)
		}
	}
}

func TestGridLinkOf(t *testing.T) {
	g := Grid{Links: []Link{{URL: "https://a.example"}}}
	if link, ok := g.LinkOf(Cell{Link: 1}); !ok || link.URL != "https://a.example" {
		t.Fatalf("got %+v, %v", link, ok)
	}
	for _, c := range []Cell{{}, {Link: 2}, {Link: -1}} {
		if link, ok := g.LinkOf(c); ok {
			t.Fatalf("cell %+v should have no link, got %+v", c, link)
		}
	}
}
