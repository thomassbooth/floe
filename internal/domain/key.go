package domain

import (
	"strings"
	"unicode"
)

// KeyMod is a set of modifier keys held during a key press.
type KeyMod uint8

const (
	ModShift KeyMod = 1 << iota
	ModAlt
	ModCtrl
	ModSuper
)

// Key is a single key press.
//
// When the key types text and ctrl is not held, Code is the typed character
// and Mod leaves out shift, so shift+h is Code 'H' with Text "H".
// Otherwise Code is the unshifted key (ctrl+b is Code 'b' with ModCtrl) or
// one of the Key* constants for special keys.
type Key struct {
	Code rune
	Text string
	Mod  KeyMod
}

// Matches reports whether two keys are the same physical press, ignoring the
// text they would produce.
func (k Key) Matches(o Key) bool {
	return k.Code == o.Code && k.Mod == o.Mod
}

// Keys that map to ASCII control characters.
const (
	KeyTab       rune = '\t'
	KeyEnter     rune = '\r'
	KeyEscape    rune = 0x1b
	KeySpace     rune = ' '
	KeyBackspace rune = 0x7f
)

// Special keys live above the Unicode range so they never collide with a
// character.
const (
	keySpecial = unicode.MaxRune + 1 + iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDown
	KeyInsert
	KeyDelete
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

var keyNames = map[rune]string{
	KeyTab:       "tab",
	KeyEnter:     "enter",
	KeyEscape:    "esc",
	KeySpace:     "space",
	KeyBackspace: "backspace",
	KeyUp:        "up",
	KeyDown:      "down",
	KeyLeft:      "left",
	KeyRight:     "right",
	KeyHome:      "home",
	KeyEnd:       "end",
	KeyPgUp:      "pgup",
	KeyPgDown:    "pgdown",
	KeyInsert:    "insert",
	KeyDelete:    "delete",
	KeyF1:        "f1",
	KeyF2:        "f2",
	KeyF3:        "f3",
	KeyF4:        "f4",
	KeyF5:        "f5",
	KeyF6:        "f6",
	KeyF7:        "f7",
	KeyF8:        "f8",
	KeyF9:        "f9",
	KeyF10:       "f10",
	KeyF11:       "f11",
	KeyF12:       "f12",
}

// String renders the key the way it is written in help text, such as
// "ctrl+b", "shift+left" or "|".
func (k Key) String() string {
	var b strings.Builder
	for _, m := range []struct {
		mod  KeyMod
		name string
	}{{ModCtrl, "ctrl"}, {ModAlt, "alt"}, {ModSuper, "super"}, {ModShift, "shift"}} {
		if k.Mod&m.mod != 0 {
			b.WriteString(m.name)
			b.WriteByte('+')
		}
	}
	if name, ok := keyNames[k.Code]; ok {
		b.WriteString(name)
	} else {
		b.WriteRune(k.Code)
	}
	return b.String()
}
