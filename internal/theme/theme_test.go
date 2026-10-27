package theme

import (
	"image/color"
	"math"
	"testing"
)

func TestCatalogNamesAreUniqueAndLookupWorks(t *testing.T) {
	seen := map[string]bool{}
	for _, th := range All() {
		if th.Name == "" || th.Label == "" {
			t.Errorf("theme %+v needs a name and a label", th)
		}
		if seen[th.Name] {
			t.Errorf("duplicate theme %q", th.Name)
		}
		seen[th.Name] = true
		if got, ok := Lookup(th.Name); !ok || got.Label != th.Label {
			t.Errorf("lookup %q failed", th.Name)
		}
	}
	if _, ok := Lookup("no-such-theme"); ok {
		t.Error("unknown themes should not be found")
	}
	if !MustLookup(Default).Dark || MustLookup(DefaultLight).Dark {
		t.Error("the defaults should be one dark and one light theme")
	}
}

// TestCatalogIsReadable checks every theme's text against its backgrounds
// with the WCAG contrast ratio: 4.5 for body text, 3 for secondary text.
func TestCatalogIsReadable(t *testing.T) {
	for _, th := range All() {
		checks := []struct {
			what   string
			fg, bg color.Color
			min    float64
		}{
			{"status bar text", th.BarFg, th.BarBg, 4.5},
			{"focused tab text", th.BarFg, th.ActiveTabBg, 4.5},
			{"hints", th.BarDim, th.BarBg, 3},
			{"badge text", th.BadgeFg, th.Accent, 3},
		}
		for _, c := range checks {
			if got := contrast(c.fg, c.bg); got < c.min {
				t.Errorf("%s: %s contrast %.2f, want at least %.1f", th.Name, c.what, got, c.min)
			}
		}
	}
}

func TestHexRejectsMalformedColors(t *testing.T) {
	for _, bad := range []string{"", "#12345", "123456", "#GGGGGG", "#1234567"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("hex(%q) should panic", bad)
				}
			}()
			hex(bad)
		}()
	}
	if got := hex("#FF8000").(color.RGBA); got != (color.RGBA{R: 0xFF, G: 0x80, A: 0xFF}) {
		t.Errorf("got %+v", got)
	}
}

func contrast(a, b color.Color) float64 {
	la, lb := luminance(a), luminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	lin := func(v uint32) float64 {
		s := float64(v) / 0xFFFF
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}
