package evalgate

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// scoutReportName is the artifact the scout phase writes into the workspace,
// resolved from the phasecontract registry (the artifact-name SSOT) rather than
// re-declared here — a registry rename must not leave this gate reading a file
// nobody writes any more.
var scoutReportName = phasecontract.ArtifactName(string(core.PhaseScout))

// materializationGate (Gate A) enforces the scout contract: every slug scout
// SELECTED must have a real .evolve/evals/<slug>.md file. It fires after the
// scout phase, before triage/tdd/build spend tokens (cycle-166).
type materializationGate struct{}

func (materializationGate) name() string                { return "evals-materialized" }
func (materializationGate) appliesTo(phase string) bool { return phase == string(core.PhaseScout) }

func (g materializationGate) check(in core.ReviewInput) (string, bool) {
	missing := g.missingSlugs(in)
	if len(missing) == 0 {
		return "", false
	}
	return "scout did not materialize evals for selected slug(s): " + strings.Join(missing, ", "), true
}

// missingSlugs returns the SELECTED slugs with no eval file on disk. Single
// source for check and remediation: the two must never disagree about which
// slugs are missing, or the agent is told to create a file the gate is not
// looking for. Empty on every fail-open path (no report, no slugs).
func (materializationGate) missingSlugs(in core.ReviewInput) []string {
	report, ok := readScoutReport(in.Workspace)
	if !ok {
		return nil // no report to parse → fail-open
	}
	slugs := SelectedSlugs(report)
	if len(slugs) == 0 {
		return nil // convergence / parse-empty → fail-open (no claim of work)
	}
	var missing []string
	for _, s := range slugs {
		if _, found := evalFilePath(in.ProjectRoot, in.Workspace, s); !found {
			missing = append(missing, s)
		}
	}
	return missing
}

// readScoutReport reads <workspace>/scout-report.md. ok is false when the file
// is absent or unreadable (callers fail open).
func readScoutReport(workspace string) (string, bool) {
	if workspace == "" {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(workspace, scoutReportName))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// evalFilePath resolves the on-disk eval file for slug, checking the project
// root first (where scout writes evals per its contract) then the workspace as
// a fallback. Returns the resolved path and whether it exists.
func evalFilePath(projectRoot, workspace, slug string) (string, bool) {
	for _, root := range []string{projectRoot, workspace} {
		if root == "" {
			continue
		}
		p := filepath.Join(root, ".evolve", "evals", slug+".md")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, true
		}
	}
	return "", false
}

// remediation tells the agent HOW to satisfy this gate: the exact path per
// missing slug, plus the grader requirement so the file it creates does not
// simply fail the NEXT gate (predicate-quality).
//
// It exists because check() reports slug STEMS while evalFilePath knows the
// directories, and the correction directive that wraps a bare stem then tells
// the agent not to create files at all. Every scout|gate-block failure in
// recorded history is this gate, each rejected after two corrections — 0-for-4
// recovery. Naming the path is what makes the rejection actionable.
//
// Only the WORKSPACE path is offered. evalFilePath also accepts an eval under
// projectRoot, but naming that would repeat this PR's own defect at a smaller
// scale: under the OS sandbox projectRoot is RepoRoot, which is explicitly
// deny-write (adapters/sandbox: ReadOnlyRepo, with only worktree/workspace/tmp
// re-permitted), so an agent told to write there would EPERM. The workspace path
// is both writable and sufficient on its own to satisfy the gate.
func (g materializationGate) remediation(in core.ReviewInput) string {
	missing := g.missingSlugs(in)
	if len(missing) == 0 {
		return ""
	}
	paths := make([]string, 0, len(missing))
	for _, s := range missing {
		paths = append(paths, filepath.Join(in.Workspace, ".evolve", "evals", s+".md"))
	}
	return "Create the missing eval file(s) — this requires writing NEW files, which is required here:\n  " +
		strings.Join(paths, "\n  ") +
		"\nEach must contain at least one `[code]` grader and test BEHAVIOR, not existence. " +
		"Write them at exactly those paths: an eval written only into the cycle worktree is NOT " +
		"visible to this gate. " +
		"Leave scout-report.md's selected slugs unchanged — the report itself is not the defect."
}
