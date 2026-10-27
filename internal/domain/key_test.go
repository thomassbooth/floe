package domain

import "testing"

func TestParseKey(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Key
	}{
		{"a", Key{Code: 'a', Text: "a"}},
		{"H", Key{Code: 'H', Text: "H"}},
		{"shift+h", Key{Code: 'H', Text: "H"}},
		{"|", Key{Code: '|', Text: "|"}},
		{"+", Key{Code: '+', Text: "+"}},
		{"-", Key{Code: '-', Text: "-"}},
		{"ctrl+b", Key{Code: 'b', Mod: ModCtrl}},
		{"Ctrl+B", Key{Code: 'b', Mod: ModCtrl | ModShift}},
		{"ctrl++", Key{Code: '+', Mod: ModCtrl}},
		{"alt+x", Key{Code: 'x', Text: "x", Mod: ModAlt}},
		{"shift+left", Key{Code: KeyLeft, Mod: ModShift}},
		{"shift+tab", Key{Code: KeyTab, Mod: ModShift}},
		{"esc", Key{Code: KeyEscape}},
		{"space", Key{Code: KeySpace}},
		{"F5", Key{Code: KeyF5}},
		{"é", Key{Code: 'é', Text: "é"}},
	} {
		got, err := ParseKey(tc.in)
		if err != nil {
			t.Errorf("ParseKey(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseKey(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

func TestParseKeyRejects(t *testing.T) {
	for _, in := range []string{"", "a+", "hyper+a", "ab", "shift+1", "ctrl+"} {
		if k, err := ParseKey(in); err == nil {
			t.Errorf("ParseKey(%q) = %#v, want an error", in, k)
		}
	}
}

func TestParseKeyReadsWhatStringWrites(t *testing.T) {
	for _, k := range []Key{
		{Code: 'b', Mod: ModCtrl},
		{Code: KeyRight, Mod: ModShift},
		{Code: KeyTab, Mod: ModShift},
		{Code: KeyEnter},
		{Code: '%', Text: "%"},
		{Code: 'q', Mod: ModCtrl | ModAlt},
	} {
		got, err := ParseKey(k.String())
		if err != nil || !got.Matches(k) {
			t.Errorf("ParseKey(%q) = %#v, %v; want %#v", k.String(), got, err, k)
		}
	}
}
