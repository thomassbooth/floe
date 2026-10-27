package e2e

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sameColor compares colors by value, whatever their concrete types.
func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	r1, g1, b1, _ := a.RGBA()
	r2, g2, b2, _ := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2
}

var (
	violet  = color.RGBA{R: 0x8B, G: 0x5C, B: 0xF6, A: 0xFF}
	mauve   = color.RGBA{R: 0xCB, G: 0xA6, B: 0xF7, A: 0xFF} // Catppuccin Mocha
	latte   = color.RGBA{R: 0x88, G: 0x39, B: 0xEF, A: 0xFF} // Catppuccin Latte
	tokyoBl = color.RGBA{R: 0x7A, G: 0xA2, B: 0xF7, A: 0xFF} // Tokyo Night
)

// bgAt returns the background color of the cell at x, y.
func (tm *term) bgAt(x, y int) color.Color {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if c := tm.emu.CellAt(x, y); c != nil {
		return c.Style.Bg
	}
	return nil
}

// rowHasBackground reports whether any cell on row y has background c.
func (tm *term) rowHasBackground(y int, c color.Color) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for x := range tm.emu.Width() {
		if cell := tm.emu.CellAt(x, y); cell != nil && sameColor(cell.Style.Bg, c) {
			return true
		}
	}
	return false
}

// accentIs waits until the focused frame's corner has the given color.
func (tm *term) accentIs(desc string, c color.Color) {
	tm.t.Helper()
	tm.waitUntil(desc, func(string) bool { return sameColor(tm.fg(0, 0), c) })
}

func TestThemePickerPreviewsKeepsAndRemembers(t *testing.T) {
	config := t.TempDir()
	tm := launchWith(t, options{w: 100, h: 30, config: config})
	tm.waitFor(" 1 sh ")
	tm.accentIs("the default violet theme", violet)

	tm.prefix("t")
	tm.waitFor("enter keep")
	t.Logf("picker:\n%s", tm.screen())
	for _, name := range []string{"Violet", "Tokyo Night", "Catppuccin Latte", "Rosé Pine"} {
		if !strings.Contains(tm.screen(), name) {
			t.Errorf("picker is missing %s", name)
		}
	}

	// Each row shows swatches of its theme's colors.
	for i, l := range tm.lines() {
		if strings.Contains(l, "Tokyo Night") && !tm.rowHasBackground(i, tokyoBl) {
			t.Errorf("Tokyo Night's row should have a swatch of its accent")
		}
		if strings.Contains(l, "Catppuccin Latte") && !tm.rowHasBackground(i, latte) {
			t.Errorf("Catppuccin Latte's row should have a swatch of its accent")
		}
	}

	tm.send("j") // previews Catppuccin Mocha on the whole screen
	tm.accentIs("a Catppuccin Mocha preview", mauve)
	tm.send("jj") // down to Tokyo Night
	tm.accentIs("a Tokyo Night preview", tokyoBl)
	tm.send("\r")
	tm.waitUntil("picker to close", func(s string) bool { return !strings.Contains(s, "enter keep") })

	raw, err := os.ReadFile(filepath.Join(config, "mux", "settings.json"))
	if err != nil || !strings.Contains(string(raw), `"tokyo-night"`) {
		t.Fatalf("the choice should be saved, got %q, %v", raw, err)
	}

	// A new run starts with the saved theme.
	tm.prefix("q")
	tm.waitExit()
	again := launchWith(t, options{w: 100, h: 30, config: config})
	again.waitFor(" 1 sh ")
	again.accentIs("the remembered theme", tokyoBl)
}

func TestThemePickerCancelGoesBack(t *testing.T) {
	config := t.TempDir()
	tm := launchWith(t, options{w: 100, h: 30, config: config})
	tm.waitFor(" 1 sh ")
	tm.prefix("t")
	tm.waitFor("enter keep")
	tm.send("j")
	tm.accentIs("a preview", mauve)
	tm.send("\x1b")
	tm.accentIs("the original theme back", violet)
	if _, err := os.Stat(filepath.Join(config, "mux", "settings.json")); err == nil {
		t.Fatal("cancelling must not save anything")
	}
}

func TestLightTerminalStartsWithLightTheme(t *testing.T) {
	tm := launchWith(t, options{w: 100, h: 30, config: t.TempDir(), background: color.White})
	tm.waitFor(" 1 sh ")
	tm.accentIs("the light theme", latte)
}

func TestSavedThemeWinsOverTerminalBackground(t *testing.T) {
	config := t.TempDir()
	os.MkdirAll(filepath.Join(config, "mux"), 0o755)
	os.WriteFile(filepath.Join(config, "mux", "settings.json"), []byte(`{"theme":"tokyo-night"}`), 0o644)
	tm := launchWith(t, options{w: 100, h: 30, config: config, background: color.White})
	tm.waitFor(" 1 sh ")
	tm.accentIs("the saved theme", tokyoBl)
}
