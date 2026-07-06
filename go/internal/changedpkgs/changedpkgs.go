// Package changedpkgs derives the set of Go package patterns touched by a
// cycle, from the builder's handoff-build.json, so an EGPS predicate can run
// `go test` scoped to the changed packages (O(change)) instead of the whole
// repo (`go test ./...`, O(repo)) — the latter exceeds the per-predicate
// timeout on a large repo and flakes to a false RED (cycle-200; the
// EVOLVE_ACS_PREDICATE_TIMEOUT_S band-aid only widens the window).
//
// The package list is exported to predicates as the CHANGED_PACKAGES env var
// (a Go predicate can scope its `go test` to it). All functions are pure +
// best-effort: an absent/unparseable handoff yields an empty list (the predicate
// then falls back to its own scope), never an error.
package changedpkgs

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
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
	// A package pattern with whitespace would be word-split by the bash consumer
	// (assert_go_test_pass_changed iterates $CHANGED_PACKAGES unquoted) into bogus
	// tokens → a false-RED predicate. Go package dirs never contain whitespace;
	// reject defensively rather than emit a splittable pattern.
	if strings.ContainsAny(p, " \t\n") {
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

// FromGit derives the changed-package set deterministically from git — the
// Rule-5 replacement for the LLM-emitted handoff-build.json, which has been
// extinct since ~cycle 215 and left the apicover CI-parity gate silently
// fail-open on every real cycle. It returns the sorted, deduped go test patterns
// for .go files that differ between baseRef and the working tree: tracked
// modifications (`git diff --name-only <baseRef>`) plus untracked new files
// (`git ls-files --others --exclude-standard`), each mapped through
// FileToPackage. Best-effort like the rest of this package: any git error yields
// an empty list (the caller falls back), never a panic.
func FromGit(repoRoot, baseRef string) []string {
	if repoRoot == "" || baseRef == "" {
		return nil
	}
	g := gitexec.Default(repoRoot)
	ctx := context.Background()
	set := map[string]struct{}{}
	add := func(out string) {
		for _, f := range strings.Split(out, "\n") {
			if f = strings.TrimSpace(f); f == "" {
				continue
			}
			if pkg, ok := FileToPackage(f); ok {
				set[pkg] = struct{}{}
			}
		}
	}
	if out, err := g.Output(ctx, "diff", "--name-only", baseRef); err == nil {
		add(out)
	}
	if out, err := g.Output(ctx, "ls-files", "--others", "--exclude-standard"); err == nil {
		add(out)
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
