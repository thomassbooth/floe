package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// writeConfig puts a floe.conf in the config directory config.
func writeConfig(t *testing.T, config, text string) {
	t.Helper()
	dir := filepath.Join(config, "floe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "floe.conf"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// frames watches the screen until done reports true, returning every
// distinct screen seen on the way, so tests can look at the frames of an
// animation.
func (tm *term) frames(desc string, done func(screen string) bool) []string {
	tm.t.Helper()
	var seen []string
	deadline := time.Now().Add(10 * time.Second)
	for {
		s := tm.screen()
		if len(seen) == 0 || seen[len(seen)-1] != s {
			seen = append(seen, s)
		}
		if done(s) {
			return seen
		}
		if time.Now().After(deadline) {
			tm.t.Fatalf("timed out waiting for %s. Screen:\n%s", desc, s)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// topRightCorner returns the column of window 1's top right corner, which
// starts at the left of the screen in these tests.
func topRightCorner(screen string) int {
	first, _, _ := strings.Cut(screen, "\n")
	return column(first, "╮")
}

// splitLayoutDone reports whether a 100 column screen shows windows 1 and 2
// side by side, the way a split right leaves them.
func splitLayoutDone(screen string) bool {
	first, _, _ := strings.Cut(screen, "\n")
	return strings.HasPrefix(first, "╭─ 1 sh ") && strings.Contains(first, "╮╭─ 2 sh ")
}

func TestSplitAnimatesWindowsIntoPlace(t *testing.T) {
	config := t.TempDir()
	writeConfig(t, config, "animations on\n")
	tm := launchWith(t, options{w: 100, h: 30, config: config, animate: true})
	tm.waitFor(" 1 sh ")
	tm.waitPrompts(1)
	start := topRightCorner(tm.screen())

	tm.prefix("|")
	// Keys typed while windows move go to the new window right away.
	tm.typeLine("echo typed-while-moving")
	seen := tm.frames("the split to settle", splitLayoutDone)
	end := topRightCorner(seen[len(seen)-1])

	var between []int
	for _, s := range seen {
		if c := topRightCorner(s); c > end && c < start {
			between = append(between, c)
		}
	}
	t.Logf("window 1's right edge went %d → %v → %d over %d frames", start, between, end, len(seen))
	t.Logf("a frame on the way:\n%s", seen[len(seen)/2])
	if len(slices.Compact(between)) < 2 {
		t.Fatalf("window 1 should shrink through several widths, not jump from %d to %d", start, end)
	}

	tm.waitFor("typed-while-moving")
	for _, l := range tm.lines() {
		if strings.Contains(l, "typed-while-moving") && column(l, "typed-while-moving") < 50 {
			t.Fatalf("typing during the animation landed in the wrong window: %q", l)
		}
	}
}

func TestAnimationsOffMovesWindowsAtOnce(t *testing.T) {
	config := t.TempDir()
	writeConfig(t, config, "# No motion, please.\nanimations off\n")
	tm := launchWith(t, options{w: 100, h: 30, config: config, animate: true})
	tm.waitFor(" 1 sh ")
	tm.prefix("|")
	seen := tm.frames("window 2", func(s string) bool { return strings.Contains(s, " 2 sh ") })
	if last := seen[len(seen)-1]; !splitLayoutDone(last) {
		t.Fatalf("the split should show in its final layout at once:\n%s", last)
	}
}

func TestBrokenConfigSaysWhichLine(t *testing.T) {
	config := t.TempDir()
	writeConfig(t, config, "animations on\nanimation off\n")
	tm := launchWith(t, options{w: 120, h: 30, config: config, animate: true})
	tm.waitFor(`floe.conf line 2: unknown setting "animation"`)
	t.Logf("screen:\n%s", tm.screen())
}

// titleRow returns the row of window 2's title, or -1.
func titleRow(screen string) int {
	for i, l := range strings.Split(screen, "\n") {
		if strings.Contains(l, " 2 sh ") && !strings.HasPrefix(strings.TrimSpace(l), "1 ") {
			return i
		}
	}
	return -1
}

func TestDroppedWindowGlidesIntoItsSnapZone(t *testing.T) {
	config := t.TempDir()
	writeConfig(t, config, "animations on\n")
	tm := launchWith(t, options{w: 100, h: 30, config: config, animate: true})
	tm.waitFor(" 1 sh ")
	tm.prefix("c")
	tm.waitCell(25, 7, "╭")
	tm.arrange()

	// Drag window 2 by its title to the middle of the left edge. It follows
	// the mouse exactly, with no animation lagging behind.
	tm.press(40, 7)
	tm.moveTo(20, 10)
	tm.waitUntil("window 2 to follow the mouse", func(s string) bool { return titleRow(s) == 10 })
	tm.moveTo(0, 14)
	tm.waitCell(0, 0, "╔")
	dropped := titleRow(tm.screen())
	tm.release(0, 14)

	seen := tm.frames("window 2 to fill the left half", func(s string) bool {
		first, _, _ := strings.Cut(s, "\n")
		return strings.HasPrefix(first, "╭┄ 2 sh ") && column(first, "╮") == 49
	})
	var between []int
	for _, s := range seen {
		if r := titleRow(s); r > 0 && r < dropped {
			between = append(between, r)
		}
	}
	t.Logf("window 2's title went from row %d → %v → 0", dropped, slices.Compact(between))
	if len(slices.Compact(between)) < 2 {
		t.Fatalf("window 2 should glide up into the zone, not jump from row %d", dropped)
	}
}
