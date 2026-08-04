package changedpkgs

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gopkgpattern"
)

// CoveringTests maps go-test package patterns — the exact strings this package
// already emits (ChangedPackages/FromGit: "./internal/foo/...") and the bare
// form core's changedGoTestPackages emits ("./internal/foo") — to the sorted,
// deduped, repo-relative slash-separated paths of the _test.go files living in
// those packages.
//
// Why it exists: test-amplification is the pipeline's per-run context outlier
// (5.4M cache-read tokens/run) because its phase spec hands the agent only the
// contract + build report, and the agent is forbidden from reading the diff, so
// it Greps the WHOLE repo to discover which existing tests cover the changed
// paths. Handing it the covering-test set removes the search. A list of test
// FILE PATHS is not the diff and not the implementation, so the black-box
// constraint is untouched.
//
// Fail-open like the rest of this package: every unusable input yields nil —
// never an error, never a panic — and the phase then behaves exactly as it does
// today. The module-wide "./..." pattern is treated as unusable on purpose:
// injecting every test file in the repo is strictly worse than today's blind
// search, so it must fail open rather than "succeed" hugely. Patterns escaping
// the module dir are rejected for the same reason a bogus `go test ./x/...`
// pattern is rejected in FileToPackage — worst case we under-scope.
func CoveringTests(repoRoot string, pkgPatterns []string) []string {
	if strings.TrimSpace(repoRoot) == "" || len(pkgPatterns) == 0 {
		return nil
	}
	moduleDir := filepath.Join(repoRoot, "go")
	seen := map[string]struct{}{}
	for _, pat := range pkgPatterns {
		rel, ok := patternDir(pat)
		if !ok {
			continue
		}
		dir := filepath.Join(moduleDir, filepath.FromSlash(rel))
		// Best-effort walk: an unreadable subtree yields no files rather than an
		// error, so a permissions blip degrades to today's behavior.
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(info.Name(), "_test.go") {
				return nil
			}
			r, rerr := filepath.Rel(repoRoot, p)
			if rerr != nil {
				return nil
			}
			seen[filepath.ToSlash(r)] = struct{}{}
			return nil
		})
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// patternDir normalizes one go-test package pattern to its module-relative
// directory, or reports false when the pattern is unusable (empty, the
// module-wide sweep, not a "./" relative pattern, or escaping the module dir).
// Both the recursive ("./internal/foo/...") and bare ("./internal/foo") forms
// map to the same directory: the walk below is recursive either way, which is
// what "/..." means to go test and is a superset — never a miss — for the bare
// form, and over-listing a sub-package's tests is not a correctness hazard for
// an advisory corpus.
func patternDir(pat string) (string, bool) {
	p := strings.TrimSpace(pat)
	if p == "" || p == gopkgpattern.WholeModule || !strings.HasPrefix(p, "./") {
		return "", false
	}
	p = strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/...")
	p = path.Clean(p)
	if p == "" || p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}
