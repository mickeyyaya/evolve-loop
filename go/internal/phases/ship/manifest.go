package ship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// manifest.go — ship-bind tree-manifest reconciliation (SHADOW).
//
// Cycle-653 second seam: ship binds the whole `git diff HEAD` tree, so any
// path present in the worktree ships (or blocks) regardless of whether the
// cycle's build/TDD phases declared it. This reconciles the paths ship is
// about to bind against the cycle's DECLARED file manifest (paths named in
// build-report.md + test-report.md) and reports out-of-manifest paths.
//
// minimal: shadow-only (log lines, never blocks) — the deliberate ceiling.
// The provisioning seam (core.ensureCleanWorktree) is the blocking arm of the
// cycle-653 fix; flipping this to enforce/targeted-stage is ONE future
// mechanism converging with inbox `ship-stage-explicit-paths` (per the inbox
// item: "implement as ONE mechanism, not two") and gets a policy.json block
// when it flips — a mode knob with only shadow implemented would be flag
// sprawl (no-feature-flags rule). ReportSizeGate precedent: new gates default
// shadow.

// manifestReportFiles are the phase reports whose named paths constitute the
// cycle's declared file manifest.
var manifestReportFiles = []string{"build-report.md", "test-report.md"}

// pathToken matches repo-relative path-like tokens (must contain a '/'; may
// carry an extension). Conservative on purpose: shadow mode tolerates a loose
// match, and a too-tight one would spam false out-of-manifest reports.
var pathToken = regexp.MustCompile(`[A-Za-z0-9_.][A-Za-z0-9_.-]*(?:/[A-Za-z0-9_.-]+)+`)

// extractReportPaths pulls the repo-relative path tokens out of a phase
// report's markdown (tables, backticks, JSON blocks all reduce to tokens).
func extractReportPaths(md string) []string {
	seen := map[string]bool{}
	for _, m := range pathToken.FindAllString(md, -1) {
		m = strings.Trim(m, ".")
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

// outOfManifest returns the changed paths not covered by the manifest, where
// a manifest entry covers a changed path exactly or as a directory prefix.
func outOfManifest(changed, manifest []string) []string {
	var extras []string
	for _, c := range changed {
		covered := false
		for _, m := range manifest {
			if c == m || strings.HasPrefix(c, strings.TrimSuffix(m, "/")+"/") {
				covered = true
				break
			}
		}
		if !covered {
			extras = append(extras, c)
		}
	}
	sort.Strings(extras)
	return extras
}

// reconcileManifestShadow reports (never blocks) the paths ship is about to
// bind that no phase report declared. Changed set = worktree porcelain dirt
// plus commits already on the cycle branch (same inputs detectColliders
// binds). Skips loudly when the workspace has no readable reports.
func reconcileManifestShadow(ctx context.Context, opts *Options, res *RunResult, worktree, branch, cycleBranch string) {
	if opts.WorkspacePath == "" {
		return
	}
	manifest := declaredManifest(opts.WorkspacePath)
	if len(manifest) == 0 {
		res.Logs = append(res.Logs, "[ship] manifest-shadow: no readable phase reports in workspace — reconciliation skipped")
		return
	}
	changedSet := map[string]bool{}
	if out, err := captureGitOutputAtDir(ctx, opts, worktree, "status", "--porcelain", "-uall"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if len(line) > 3 {
				changedSet[strings.Trim(strings.TrimSpace(line[3:]), `"`)] = true
			}
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
	if extras := outOfManifest(changed, manifest); len(extras) > 0 {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] manifest-shadow: %d out-of-manifest path(s) about to be bound (would block under enforce): %s", len(extras), strings.Join(extras, ", ")))
		return
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] manifest-shadow: OK — all %d bound path(s) covered by the declared build/TDD manifest", len(changed)))
}
