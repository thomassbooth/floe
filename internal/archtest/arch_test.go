// Package archtest enforces the layering rules: the domain is pure, the
// service depends only inward, and only the UI and infrastructure adapters
// may import third-party libraries such as Bubble Tea.
package archtest

import (
	"encoding/json"
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/thomasbooth/multiplexer"

// rules lists, for each package, the project packages it may import and
// whether it may import anything outside the standard library.
var rules = map[string]struct {
	project    []string
	thirdParty bool
}{
	"internal/domain":           {},
	"internal/queue":            {},
	"internal/theme":            {},
	"internal/infra/atomicfile": {},
	"internal/infra/settings":   {project: []string{"internal/infra/atomicfile"}},
	"internal/infra/store":      {project: []string{"internal/domain", "internal/infra/atomicfile"}},
	"internal/infra/socket":     {},
	"internal/input":            {project: []string{"internal/domain"}},
	"internal/protocol":         {project: []string{"internal/domain"}},
	"internal/remote":           {project: []string{"internal/domain", "internal/protocol", "internal/queue"}},
	"internal/service":          {project: []string{"internal/domain", "internal/queue"}},
	"internal/infra/terminal":   {project: []string{"internal/domain", "internal/queue"}, thirdParty: true},
	"internal/ui":               {project: []string{"internal/domain", "internal/input", "internal/theme"}, thirdParty: true},
	"cmd/mux":                   {project: []string{"internal/domain", "internal/infra/settings", "internal/infra/socket", "internal/infra/store", "internal/infra/terminal", "internal/input", "internal/remote", "internal/service", "internal/ui"}, thirdParty: true},
	"internal/archtest":         {},
}

type pkg struct {
	ImportPath string
	Imports    []string
}

func TestLayering(t *testing.T) {
	out, err := exec.Command("go", "list", "-json", module+"/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	stdlib := map[string]bool{}
	stdOut, err := exec.Command("go", "list", "std").Output()
	if err != nil {
		t.Fatalf("go list std: %v", err)
	}
	for _, p := range strings.Fields(string(stdOut)) {
		stdlib[p] = true
	}

	dec := json.NewDecoder(strings.NewReader(string(out)))
	seen := 0
	for {
		var p pkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p.ImportPath, module), "/")
		if rel == "e2e" {
			continue // black-box tests may use anything
		}
		rule, ok := rules[rel]
		if !ok {
			t.Errorf("package %s has no layering rule; add one to archtest", rel)
			continue
		}
		seen++
		for _, imp := range p.Imports {
			switch {
			case stdlib[imp]:
			case strings.HasPrefix(imp, module+"/"):
				dep := strings.TrimPrefix(imp, module+"/")
				if !slices.Contains(rule.project, dep) {
					t.Errorf("%s must not import %s", rel, dep)
				}
			case !rule.thirdParty:
				t.Errorf("%s must not import third-party package %s", rel, imp)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no packages checked")
	}
}
