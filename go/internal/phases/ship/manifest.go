package ship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// manifest.go — ship-bind tree-manifest reconciliation (shadow + enforce).
//
// Cycle-653 second seam: ship binds the whole `git diff HEAD` tree, so any
// path present in the worktree ships (or blocks) regardless of whether the
// cycle's build/TDD phases declared it. reconcileManifest reconciles the paths
// ship is about to bind against the cycle's DECLARED file manifest (paths named
// in build-report.md + test-report.md).
//
// Two modes (opts.ManifestGate, config-sourced; default shadow — the
// ReportSizeGate "new gates default shadow" precedent):
//   - shadow (default): log out-of-manifest paths, never block — behavior-preserving.
//   - enforce: FAIL CLOSED on any out-of-manifest path — the cross-lane
//     untracked-leak guard (inbox `ship-stage-explicit-paths`, cycle-645).
//
// BEFORE enabling enforce in production (the deferred policy.json→Options
// wiring), prerequisites (2026-07-14 review):
//   1. DONE (2026-07-14): pathToken (below) now also extracts bare root-level
//      filenames (CHANGELOG.md, go.mod) via an extension allow-list, so a legit
//      root-file change is no longer a FALSE-BLOCK under enforce.
//   2. DONE (cycle-1064): the enforce branch carries the dedicated
//      core.CodeManifestGate (mirroring CodeCommitPrefixGate) instead of reusing
//      CodeGitStageFailed, so the ledger/debugger can tell a manifest block from
//      a real `git add` failure; router.shipLocalCodes routes it to the debugger.
//   3. DONE (cycle-1064): policy.json `gates.manifest_gate` resolves through
//      policy.GatesConfig() into ship.Config.ManifestGate → Options.ManifestGate,
//      so enforce is operator-activatable without a code edit. Default stays
//      "shadow" — behavior-preserving.

// manifestReportFiles are the phase reports whose named paths constitute the
// cycle's declared file manifest.
var manifestReportFiles = []string{"build-report.md", "test-report.md"}

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
		m := strings.Trim(md[start:end], ".")
		if m != "" {
			seen[m] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// declaredManifest is the union of paths named in the workspace's phase
// reports. Empty when no report is readable — the caller must then skip
// reconciliation (no manifest ≠ empty manifest).
func declaredManifest(workspacePath string) []string {
	seen := map[string]bool{}
	for _, f := range manifestReportFiles {
		b, err := os.ReadFile(filepath.Join(workspacePath, f))
		if err != nil {
			continue
		}
		for _, p := range extractReportPaths(string(b)) {
			seen[p] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// manifestCovers reports whether some manifest entry covers p — exactly, or as
// a directory prefix. SSOT for the coverage predicate shared by the gate
// (outOfManifest) and explicit staging (stagePathspec).
func manifestCovers(manifest []string, p string) bool {
	for _, m := range manifest {
		if p == m || strings.HasPrefix(p, strings.TrimSuffix(m, "/")+"/") {
			return true
		}
	}
	return false
}

// outOfManifest returns the changed paths not covered by the manifest, where
// a manifest entry covers a changed path exactly or as a directory prefix.
func outOfManifest(changed, manifest []string) []string {
	var extras []string
	for _, c := range changed {
		if !manifestCovers(manifest, c) {
			extras = append(extras, c)
		}
	}
	sort.Strings(extras)
	return extras
}

// porcelainChangedPaths parses `git status --porcelain` output into the sorted
// set of repo-relative paths it names. A rename entry ("R  old -> new") yields
// BOTH sides, so an explicit staging pathspec records the deletion as well as
// the addition.
func porcelainChangedPaths(out string) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if len(line) <= 3 {
			continue
		}
		for _, part := range strings.Split(line[3:], " -> ") {
			if p := strings.Trim(strings.TrimSpace(part), `"`); p != "" {
				seen[p] = true
			}
		}
	}
	return sortedKeys(seen)
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

// stagePathspec computes the explicit `git add -- <paths>` pathspec for a
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
// Fallbacks — staging must never silently become a no-op, which would produce a
// false clean exit / empty ship, and must never fall back to `-A`:
//   - no manifest (no workspace, or no readable phase reports) → the full
//     porcelain changed set;
//   - a manifest that covers nothing that changed → likewise the changed set.
func stagePathspec(manifest, changed []string, isFile func(string) bool) []string {
	if len(manifest) == 0 {
		return changed
	}
	changedSet := map[string]bool{}
	for _, c := range changed {
		changedSet[c] = true
	}
	staged := map[string]bool{}
	for _, d := range manifest {
		if changedSet[d] || isFile(d) {
			staged[d] = true
		}
	}
	for _, c := range changed {
		if manifestCovers(manifest, c) {
			staged[c] = true
		}
	}
	if len(staged) == 0 {
		return changed
	}
	return sortedKeys(staged)
}

// ManifestGateEnforce is the opts.ManifestGate value that switches the gate from
// shadow (log-only) to fail-closed. Any other value (including "") is shadow.
const ManifestGateEnforce = "enforce"

// manifestGateEnforced reports whether the manifest gate BLOCKS on out-of-
// manifest paths (enforce) vs only logs them (shadow, the default). Pure;
// config-sourced via opts.ManifestGate, never a code literal.
func manifestGateEnforced(mode string) bool {
	return mode == ManifestGateEnforce
}

// reconcileManifest reconciles the paths ship is about to bind against the
// cycle's DECLARED file manifest (build-report.md + test-report.md). Changed
// set = worktree porcelain dirt plus commits already on the cycle branch (same
// inputs detectColliders binds). In SHADOW mode (the default) it only REPORTS
// out-of-manifest paths and returns nil — behavior-preserving. In ENFORCE mode
// it FAILS CLOSED on any out-of-manifest path: refusing to commit files no
// phase declared, which under a fleet are typically a sibling lane's untracked
// leak (cycle-645) whose commit reddens main. A loud, recoverable block beats a
// silent contaminated commit. Returns nil (skips) when no reports are readable.
func reconcileManifest(ctx context.Context, opts *Options, res *RunResult, worktree, branch, cycleBranch string) error {
	if opts.WorkspacePath == "" {
		return nil
	}
	manifest := declaredManifest(opts.WorkspacePath)
	if len(manifest) == 0 {
		res.Logs = append(res.Logs, "[ship] manifest-gate: no readable phase reports in workspace — reconciliation skipped")
		return nil
	}
	changedSet := map[string]bool{}
	if out, err := captureGitOutputAtDir(ctx, opts, worktree, "status", "--porcelain", "-uall"); err == nil {
		for _, p := range porcelainChangedPaths(out) {
			changedSet[p] = true
		}
	}
	if out, err := captureGitOutputAtDir(ctx, opts, worktree, "diff", "--name-only", branch, cycleBranch); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				changedSet[line] = true
			}
		}
	}
	changed := make([]string, 0, len(changedSet))
	for p := range changedSet {
		changed = append(changed, p)
	}
	sort.Strings(changed)
	extras := outOfManifest(changed, manifest)
	if len(extras) == 0 {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] manifest-gate: OK — all %d bound path(s) covered by the declared build/TDD manifest", len(changed)))
		return nil
	}
	if manifestGateEnforced(opts.ManifestGate) {
		// FAIL-CLOSED — see reconcileManifest's doc + the error string below.
		return shipErr(core.CodeManifestGate, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: manifest-gate (enforce): refusing to commit %d path(s) no build/TDD report declared (likely a cross-lane untracked leak): %s", len(extras), strings.Join(extras, ", ")),
			"out_of_manifest", strings.Join(extras, ","))
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] manifest-gate: %d out-of-manifest path(s) about to be bound (SHADOW — would block under enforce): %s", len(extras), strings.Join(extras, ", ")))
	return nil
}
