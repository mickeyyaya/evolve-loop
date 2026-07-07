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
// an empty list (the caller falls back), never a panic. Callers that must
// distinguish "0 files changed" from "git failed" should use FromGitChecked.
func FromGit(repoRoot, baseRef string) []string {
	pkgs, _ := FromGitChecked(repoRoot, baseRef)
	return pkgs
}

// FromGitChecked is FromGit plus a derivability signal: it returns
// (pkgs, derivable) where derivable is false whenever the changed-package set
// could NOT be trusted — an empty repoRoot/baseRef (a config error, not a
// verified-clean tree) or ANY git invocation failing (no repo, bad baseRef, a
// concurrent-fleet `.git/index.lock` race). A clean tree that git reports
// successfully is (nil, true): genuinely nothing changed, NOT underivable.
//
// FromGit's swallow-every-error behavior conflates those two cases, which makes
// the apicover CI-parity gate fail-open on the very cycle that most needs it
// (cycle-581 audit D1/D2, warnship_apicover_ci_gap 3rd recurrence). A gate that
// must FAIL loud on an underivable set uses this; the CHANGED_PACKAGES predicate
// path keeps FromGit's best-effort empty-on-error contract.
func FromGitChecked(repoRoot, baseRef string) ([]string, bool) {
	if repoRoot == "" || baseRef == "" {
		return nil, false
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
	out, err := g.Output(ctx, "diff", "--name-only", baseRef)
	if err != nil {
		return nil, false // git failed → underivable, not "nothing changed"
	}
	add(out)
	out, err = g.Output(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, false
	}
	add(out)
	if len(set) == 0 {
		return nil, true // git succeeded, tree genuinely clean
	}
	res := make([]string, 0, len(set))
	for p := range set {
		res = append(res, p)
	}
	sort.Strings(res)
	return res, true
}
