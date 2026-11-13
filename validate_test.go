package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func load(t *testing.T, path string) *Index {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

// TestIndex checks the real plugins.json.
func TestIndex(t *testing.T) {
	ix := load(t, "plugins.json")
	for _, err := range Check(ix) {
		t.Error(err)
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ix.Plugins {
		if !strings.Contains(string(readme), "| "+e.ID+" |") {
			t.Errorf("README.md's plugin table has no row for %s", e.ID)
		}
	}
	if testing.Short() {
		t.Skip("-short: not resolving packages")
	}
	for _, e := range ix.Plugins {
		if e.Kind != "native" {
			continue
		}
		t.Run(e.ID, func(t *testing.T) {
			if err := CheckPackage(e); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestBadFixtures checks that each broken fixture is rejected for the
// right reason.
func TestBadFixtures(t *testing.T) {
	want := map[string]string{
		"duplicate-id.json":    "id is used twice",
		"missing-package.json": "native entries need a package",
		"bad-id.json":          "id must be lowercase",
		"missing-id.json":      "id is missing",
		"exec-no-command.json": "exec entries need a command",
		"unknown-field.json":   `unknown field "packge"`,
		"wrong-version.json":   "version is 2",
		"bad-kind.json":        `kind "wasm" must be native or exec`,
		"missing-fields.json":  "description is missing",
		"version-in-pkg.json":  "put the version in",
		"bad-json.json":        "plugins.json",
	}
	files, _ := filepath.Glob("testdata/bad/*.json")
	if len(files) != len(want) {
		t.Fatalf("%d fixtures, %d expectations", len(files), len(want))
	}
	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var msgs []string
			ix, err := Parse(b)
			if err != nil {
				msgs = append(msgs, err.Error())
			} else {
				for _, e := range Check(ix) {
					msgs = append(msgs, e.Error())
				}
			}
			got := strings.Join(msgs, "\n")
			if w, ok := want[name]; !ok {
				t.Fatalf("no expectation for %s", name)
			} else if !strings.Contains(got, w) {
				t.Fatalf("want an error containing %q, got:\n%s", w, got)
			}
		})
	}
}

func TestCheckPackageRejectsMissing(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: network")
	}
	err := CheckPackage(Entry{ID: "x", Kind: "native", Package: "github.com/dewald-L/lazy1on1/no-such-package"})
	if err == nil || !strings.Contains(err.Error(), "has no Go package") {
		t.Fatalf("missing package in a real module: %v", err)
	}
	err = CheckPackage(Entry{ID: "x", Kind: "native", Package: "github.com/dewald-L/lazy1on1/dockplugin", Version: "v99.0.0"})
	if err == nil {
		t.Fatal("unknown version resolved")
	}
}
