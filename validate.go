// Package plugins validates plugins.json, the index of dock plugins.
//
// `go test ./...` runs the checks on the real plugins.json (including that
// every native package resolves with the Go toolchain) and on the broken
// fixtures in testdata/bad. `go test -short` skips the network checks.
package plugins

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// FormatVersion is the plugins.json format version dock reads.
const FormatVersion = 1

// MaxSize is the largest index dock accepts.
const MaxSize = 1 << 20

// idRe is dock's plugin id rule (plugin.ValidID).
var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Index is plugins.json.
type Index struct {
	Version int     `json:"version"`
	Plugins []Entry `json:"plugins"`
}

// Entry is one plugin. See README.md for the fields.
type Entry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Kind        string   `json:"kind"`
	Package     string   `json:"package,omitempty"`
	Version     string   `json:"version,omitempty"`
	Command     []string `json:"command,omitempty"`
	Summary     []string `json:"summary,omitempty"`
	Requires    string   `json:"requires,omitempty"`
	Repo        string   `json:"repo,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Parse decodes an index strictly: unknown fields are errors here (dock
// ignores them, but in this repo they are almost always typos). Add a new
// field to Entry in the same PR that starts using it.
func Parse(b []byte) (*Index, error) {
	if len(b) > MaxSize {
		return nil, fmt.Errorf("plugins.json is %d bytes; dock reads at most %d", len(b), MaxSize)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var ix Index
	if err := dec.Decode(&ix); err != nil {
		return nil, fmt.Errorf("plugins.json: %w", err)
	}
	if dec.More() {
		return nil, errors.New("plugins.json: trailing data after the top-level object")
	}
	return &ix, nil
}

// Check returns every structural problem in ix (no network).
func Check(ix *Index) []error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if ix.Version != FormatVersion {
		bad("version is %d; this repo publishes format %d", ix.Version, FormatVersion)
	}
	seen := map[string]bool{}
	for i, e := range ix.Plugins {
		where := fmt.Sprintf("plugins[%d] (%s)", i, e.ID)
		switch {
		case e.ID == "":
			bad("%s: id is missing", where)
		case !idRe.MatchString(e.ID):
			bad("%s: id must be lowercase letters, digits and dashes", where)
		case seen[e.ID]:
			bad("%s: id is used twice", where)
		}
		seen[e.ID] = true
		if strings.TrimSpace(e.Name) == "" {
			bad("%s: name is missing", where)
		}
		if strings.TrimSpace(e.Description) == "" {
			bad("%s: description is missing", where)
		}
		if strings.ContainsAny(e.Description, "\n\r") || len(e.Description) > 120 {
			bad("%s: description must be one line of at most 120 characters", where)
		}
		switch e.Kind {
		case "native":
			if e.Package == "" {
				bad("%s: native entries need a package", where)
			}
			if strings.Contains(e.Package, "@") || strings.ContainsAny(e.Package, " \t") {
				bad("%s: package is an import path; put the version in \"version\"", where)
			}
			if len(e.Command) > 0 || len(e.Summary) > 0 {
				bad("%s: command and summary are for exec entries", where)
			}
		case "exec":
			if len(e.Command) == 0 || strings.TrimSpace(e.Command[0]) == "" {
				bad("%s: exec entries need a command", where)
			}
			if e.Package != "" || e.Version != "" {
				bad("%s: package and version are for native entries", where)
			}
		case "":
			bad("%s: kind is missing (native or exec)", where)
		default:
			bad("%s: kind %q must be native or exec", where, e.Kind)
		}
		if strings.ContainsAny(e.Version, " \t@") {
			bad("%s: version must be a tag or commit", where)
		}
		if strings.Contains(e.Requires, "\n") {
			bad("%s: requires must be one line", where)
		}
		if e.Repo != "" {
			if u, err := url.Parse(e.Repo); err != nil || u.Scheme != "https" || u.Host == "" {
				bad("%s: repo must be an https URL", where)
			}
		}
		for _, t := range e.Tags {
			if !idRe.MatchString(t) {
				bad("%s: tag %q must be lowercase letters, digits and dashes", where, t)
			}
		}
	}
	return errs
}

// CheckPackage resolves a native entry's package with the Go toolchain: it
// finds the module that contains the package (go list -m), downloads it at
// the pinned version or latest, and checks the package directory has Go
// files.
func CheckPackage(e Entry) error {
	version := e.Version
	if version == "" {
		version = "latest"
	}
	parts := strings.Split(e.Package, "/")
	var lastErr error
	for n := len(parts); n >= 1; n-- {
		mod := strings.Join(parts[:n], "/")
		out, err := goCmd("list", "-m", "-json", mod+"@"+version)
		if err != nil {
			lastErr = err
			continue
		}
		var m struct{ Path, Version string }
		if err := json.Unmarshal(out, &m); err != nil {
			return err
		}
		if m.Path != mod {
			continue
		}
		out, err = goCmd("mod", "download", "-json", m.Path+"@"+m.Version)
		if err != nil {
			return err
		}
		var dl struct{ Dir string }
		if err := json.Unmarshal(out, &dl); err != nil || dl.Dir == "" {
			return fmt.Errorf("go mod download %s@%s: no directory (%v)", m.Path, m.Version, err)
		}
		dir := filepath.Join(dl.Dir, filepath.FromSlash(strings.TrimPrefix(e.Package, mod)))
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		if len(files) == 0 {
			return fmt.Errorf("%s@%s has no Go package %s", m.Path, m.Version, e.Package)
		}
		return nil
	}
	return fmt.Errorf("no module provides %s@%s: %v", e.Package, version, lastErr)
}

func goCmd(args ...string) ([]byte, error) {
	c := exec.Command("go", args...)
	c.Dir = os.TempDir() // outside any module
	c.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	var stderr bytes.Buffer
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("go %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
