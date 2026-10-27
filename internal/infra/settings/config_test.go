package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	cfg, errs := ParseConfig("# Comment\n\n  animations   off  \n")
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if cfg.Animations == nil || *cfg.Animations {
		t.Fatalf("animations = %v, want off", cfg.Animations)
	}
	if cfg, _ := ParseConfig(""); cfg.Animations != nil {
		t.Fatal("an empty file should leave animations unset")
	}
}

func TestParseConfigReportsEveryLine(t *testing.T) {
	_, errs := ParseConfig("animations maybe\nanimations\nwobble on\n")
	var got []string
	for _, e := range errs {
		got = append(got, e.Error())
	}
	want := []string{
		"line 1: animations takes on or off",
		"line 2: animations takes on or off",
		`line 3: unknown setting "wobble"`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("errors:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestReadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "floe.conf")
	if cfg, errs := ReadConfig(path); cfg.Animations != nil || errs != nil {
		t.Fatalf("a missing file should be an empty config: %v, %v", cfg, errs)
	}

	os.WriteFile(path, []byte("animations off\nnope\n"), 0o644)
	cfg, errs := ReadConfig(path)
	if cfg.Animations != nil {
		t.Error("a broken file should not be used at all")
	}
	if len(errs) != 1 || errs[0].Error() != `floe.conf line 2: unknown setting "nope"` {
		t.Errorf("errs = %v", errs)
	}
}
