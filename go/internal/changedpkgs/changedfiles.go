package changedpkgs

import (
	"context"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// ChangedFile is one repo-relative path that differs between a base ref and
// the WORKING TREE: a tracked modification or deletion from `git diff`, or an
// untracked file from `git ls-files --others`. Added is true for a path the
// base does not have — an index-added or untracked file, or the destination
// of a rename. A rename's source is not a changed file: what it left behind
// has nothing to import.
type ChangedFile struct {
	Path  string
	Added bool
}

// ChangedFilesChecked is the ONE git derivation of "what does this tree change
// versus baseRef"; FromGitChecked and the ship gate's backstops are projections
// of it. It reads the working tree, never the index: a lane's build output is
// unstaged, and its new files untracked, until the ship itself stages them —
// an index-based seed at gate time sees nothing (the 2026-09-14 ship-gate
// incident). ok is false whenever git could not answer: empty inputs, no
// repository, a bad ref, a concurrent index.lock — never "nothing changed".
func ChangedFilesChecked(repoRoot, baseRef string) ([]ChangedFile, bool) {
	if repoRoot == "" || baseRef == "" {
		return nil, false
	}
	g := gitexec.Default(repoRoot)
	ctx := context.Background()
	out, err := g.Output(ctx, "diff", "--name-status", baseRef)
	if err != nil {
		return nil, false
	}
	var files []ChangedFile
	for _, line := range strings.Split(out, "\n") {
		// "M\tpath", "A\tpath", "D\tpath", "R100\told\tnew" — the last field
		// is the path that exists now (a rename's destination).
		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(fields) < 2 || strings.TrimSpace(fields[0]) == "" {
			continue
		}
		status := fields[0][0]
		files = append(files, ChangedFile{Path: fields[len(fields)-1], Added: status == 'A' || status == 'R' || status == 'C'})
	}
	out, err = g.Output(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, false
	}
	for _, p := range strings.Split(out, "\n") {
		if p = strings.TrimSpace(p); p != "" {
			files = append(files, ChangedFile{Path: p, Added: true})
		}
	}
	return files, true
}

// PackagesOf projects changed files onto the sorted, deduped go test patterns
// of the Go packages they live in (FileToPackage); nil when none is a Go file
// inside the module.
func PackagesOf(files []ChangedFile) []string {
	set := map[string]struct{}{}
	for _, f := range files {
		if pkg, ok := FileToPackage(f.Path); ok {
			set[pkg] = struct{}{}
		}
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
