// Package changedpkgs derives the set of Go package patterns touched by a
// cycle, from the builder's handoff-build.json, so an EGPS predicate can run
// `go test` scoped to the changed packages (O(change)) instead of the whole
// repo (`go test ./...`, O(repo)) — the latter exceeds the per-predicate
// timeout on a large repo and flakes to a false RED (cycle-200; the
// EVOLVE_ACS_PREDICATE_TIMEOUT_S band-aid only widens the window).
//
// The package list is exported to predicates as the CHANGED_PACKAGES env var;
// the acs/lib/assert.sh helper assert_go_test_pass_changed consumes it. All
// functions are pure + best-effort: an absent/unparseable handoff yields an
// empty list (the predicate then falls back to its own scope), never an error.
package changedpkgs

import (
	"encoding/json"
	"os"
	"path"
	"sort"
	"strings"
)

// FileToPackage maps a repo path to its go-module-relative test pattern
// ("./internal/foo/..."), or ("", false) for anything that is not a Go source
// file inside the module. All Go source lives under the go/ module dir, so the
// path MUST carry a leading "go/" — a .go path without it is outside the module
// and is rejected rather than turned into a bogus pattern (a non-existent
// `go test ./x/...` would false-RED a predicate). Worst case we under-scope (the
// predicate falls back to its own scope), never misroute. A .go file at the
// module root maps to "./...".
func FileToPackage(file string) (string, bool) {
	p := strings.TrimSpace(path.Clean(strings.ReplaceAll(file, "\\", "/")))
	if !strings.HasSuffix(p, ".go") {
		return "", false
	}
	p = strings.TrimPrefix(p, "./")
	if !strings.HasPrefix(p, "go/") {
		return "", false
	}
	dir := path.Dir(strings.TrimPrefix(p, "go/"))
	if dir == "." || dir == "" {
		return "./...", true // module-root file: no narrower scope
	}
	return "./" + dir + "/...", true
}

// handoffDoc mirrors the changed-file arrays the builder emits in its handoff.
type handoffDoc struct {
	Thrusts []struct {
		FilesModified []string `json:"files_modified"`
		FilesNew      []string `json:"files_new"`
	} `json:"thrusts"`
}

// ChangedPackages reads handoff-build.json at handoffPath and returns the
// deduped, sorted set of go test patterns for the Go files it lists. Missing or
// unparseable handoff → nil (best-effort).
func ChangedPackages(handoffPath string) []string {
	data, err := os.ReadFile(handoffPath)
	if err != nil {
		return nil
	}
	var d handoffDoc
	if err := json.Unmarshal(data, &d); err != nil {
		return nil
	}
	set := map[string]struct{}{}
	add := func(files []string) {
		for _, f := range files {
			if pkg, ok := FileToPackage(f); ok {
				set[pkg] = struct{}{}
			}
		}
	}
	for _, th := range d.Thrusts {
		add(th.FilesModified)
		add(th.FilesNew)
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
