// Package shipmanifest selects the paths a Ship commit binds from the declared phase-report manifest.
package shipmanifest

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// bareRootFileExts is the extension allow-list for bare root-level filenames
// (no '/'). Kept to the extensions the repo actually tracks at its root or that
// a phase report legitimately names as a bare file.
const bareRootFileExts = `go|mod|sum|md|json|ya?ml|txt|toml|lock|sh`

// pathToken matches repo-relative path-like tokens: EITHER a slashed path, OR a
// bare root-level filename carrying one of the known source/doc extensions
// (bareRootFileExts). The slash form is loose on purpose; the bare form is
// gated by an extension allow-list so prose tokens with an incidental dot
// ("cfg.Now", version "1.0", "e.g.") do NOT match — the false-positive risk a
// naive `\w+\.\w+` would create. Files with no extension (Makefile, LICENSE) or
// a leading dot (.goreleaser.yml) are out of scope: extractReportPaths's
// left-boundary check makes them a CLEAN non-match (not a truncation). Declare
// such files via a slashed path or an explicit manifest entry if enforce needs.
var pathToken = regexp.MustCompile(
	`[A-Za-z0-9_.][A-Za-z0-9_.-]*(?:/[A-Za-z0-9_.-]+)+` +
		`|[A-Za-z0-9_][A-Za-z0-9_-]*\.(?:` + bareRootFileExts + `)\b`)

// isPathContinuationByte reports whether b could be the interior of a path/word
// token (word char, '.', '-', or '/'). Used as a manual left-boundary check:
// RE2 has no lookbehind, so without it the bare-filename alternative would start
// matching INSIDE a larger token — e.g. truncating ".goreleaser.yml" to a bogus
// "goreleaser.yml" that can never cover the real dotfile path.
func isPathContinuationByte(b byte) bool {
	return b == '.' || b == '-' || b == '/' || b == '_' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// extractReportPaths pulls the repo-relative path tokens out of a phase
// report's markdown (tables, backticks, JSON blocks all reduce to tokens).
func extractReportPaths(md string) []string {
	seen := map[string]bool{}
	for _, loc := range pathToken.FindAllStringIndex(md, -1) {
		start, end := loc[0], loc[1]
		// Reject a match whose left edge sits inside a larger token (a legit path
		// token begins at string start or just after a separator). This makes
		// leading-dot files a CLEAN non-match rather than a silent truncation.
		if start > 0 && isPathContinuationByte(md[start-1]) {
			continue
		}
		// Trailing dots are sentence punctuation. Leading "./" is the explicit
		// relative prefix ("$ ./go/bin/evolve selfcheck build" — the slice-B
		// pre-flight line every build-report carries) and normalizes to the
		// repo-relative path. A blind Trim(m, ".") here turned that token into
		// the ABSOLUTE pathspec "/go/bin/evolve" → `git add` fatal rc=128
		// ("Invalid path '/go'") → cycle-1098's audited-PASS work stranded.
		m := strings.TrimRight(md[start:end], ".")
		for strings.HasPrefix(m, "./") {
			m = m[2:]
		}
		// A residual leading ".." is either a repo escape ("../x") or ellipsis
		// punctuation glued to a token ("...x/y.go", "..foo/bar") — never a
		// declarable repo path. Dropping beats the old blind Trim, which
		// mangled these into plausible-but-wrong entries.
		if strings.HasPrefix(m, "..") || !isRepoRelative(m) {
			continue // absolute or repo-escaping — never a manifest entry
		}
		seen[m] = true
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Declared is the union of paths named in the workspace's phase
// reports. Empty when no report is readable — the caller must then skip
// reconciliation (no manifest ≠ empty manifest).
func Declared(workspacePath string, reportFiles []string) []string {
	seen := map[string]bool{}
	fold := func(ws string) {
		for _, f := range reportFiles {
			b, err := os.ReadFile(filepath.Join(ws, f))
			if err != nil {
				continue
			}
			for _, p := range extractReportPaths(string(b)) {
				seen[p] = true
			}
		}
	}
	fold(workspacePath)
	// ADR-0076 slice C: a continuation cycle re-exposes the PRIOR attempt's
	// files at ship time, but this cycle's reports declare only what the
	// resuming builder re-touched. The adoption seam copies the continuation
	// manifest into this workspace; union the prior attempt's declarations
	// (sibling run dir, keyed by its cycle) so enforce mode never fails closed
	// on legitimately resumed work. One hop only — a re-FAILed continuation's
	// next manifest points at the latest attempt, whose reports accumulate the
	// same union at ITS ship.
	if c, ok, _ := continuation.ReadManifest(workspacePath); ok && c.Cycle > 0 {
		fold(filepath.Join(filepath.Dir(workspacePath), fmt.Sprintf("cycle-%d", c.Cycle)))
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// isRepoRelative reports whether p can be handed to git as a repo-relative
// pathspec. Absolute ("/go/evolve") and repo-escaping ("../x", including
// interior escapes like "a/../../etc/x" — git: "is outside repository",
// rc=128) entries are the cycle-1098 `git add` fatal class: git canonicalizes
// them against the FILESYSTEM, not the repo root. The check runs on the
// path.Clean'd form so interior ".." segments cannot smuggle an escape past a
// prefix test. SSOT for the extraction filter (extractReportPaths) and the
// staging seam (Pathspec).
func isRepoRelative(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") {
		return false
	}
	c := path.Clean(p)
	return c != "" && c != "." && !strings.HasPrefix(c, "/") &&
		c != ".." && !strings.HasPrefix(c, "../")
}

// manifestCovers reports whether some manifest entry covers p — exactly, or as
// a directory prefix. SSOT for the coverage predicate shared by the gate
// (OutOfManifest) and explicit staging (Pathspec).
func manifestCovers(manifest []string, p string) bool {
	for _, m := range manifest {
		if p == m || strings.HasPrefix(p, strings.TrimSuffix(m, "/")+"/") {
			return true
		}
	}
	return false
}

// OutOfManifest returns the changed paths not covered by the manifest, where
// a manifest entry covers a changed path exactly or as a directory prefix.
func OutOfManifest(changed, manifest []string) []string {
	var extras []string
	for _, c := range changed {
		if !manifestCovers(manifest, c) {
			extras = append(extras, c)
		}
	}
	sort.Strings(extras)
	return extras
}

// sortedKeys renders a path set as a sorted slice.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// pathspec computes the explicit `git add -- <paths>` pathspec for a
// non-release ship (cycle-1067, `ship-stage-explicit-paths`): the DECLARED
// manifest, not `git add -A`, decides what a cycle/manual ship binds — so a
// sibling lane's untracked leak (cycle-645) can no longer ride into the commit.
//
// The set is:
//   - every declared entry that is a real file on disk (isFile) or that git
//     reports as changed (so a DELETED declared path still stages its deletion);
//   - plus every changed path the manifest covers by directory prefix (a new
//     file under a declared directory is part of the declared change).
//
// No manifest, or one that covers nothing, selects nothing: the tracked edits
// ship through the audit binding's `add -u`, and adopting every changed path
// instead would carry untracked residue into the binding (cycle 1594).
func pathspec(manifest, changed []string, isFile func(string) bool) []string {
	staged := map[string]bool{}
	for _, d := range manifest {
		// Defense in depth vs extractReportPaths' own filter: a manifest entry
		// from an older serialized source (a pre-fix continuation union) must
		// still never reach git argv as an absolute/escaping pathspec — the
		// isFile probe resolves such entries INSIDE root (filepath.Join swallows
		// the leading slash), so it cannot be the guard.
		if !isRepoRelative(d) {
			continue
		}
		if isFile(d) {
			staged[d] = true
		}
	}
	// changed comes from `git status --porcelain` at the tree root — always
	// repo-relative by construction, so no isRepoRelative re-check.
	for _, c := range changed {
		if manifestCovers(manifest, c) {
			staged[c] = true
		}
	}
	return sortedKeys(staged)
}
