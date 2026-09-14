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
	ungraded := g.ungradedSlugs(in)
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "scout did not materialize evals for selected slug(s): "+strings.Join(missing, ", "))
	}
	if len(ungraded) > 0 {
		parts = append(parts, "scout materialized evals without a [code] grader for selected slug(s): "+strings.Join(ungraded, ", ")+
			" — an eval that only asserts existence caps nothing, and the lane's own durability test refuses it at the ship gate (cycle 1679)")
	}
	// Only a CERTAIN violation blocks. The advisory below is appended AFTER this
	// is latched so a parse-miss can never turn into a hard block, and a real
	// missing eval can never be downgraded by one.
	block := len(parts) > 0
	if adv := g.parseMissAdvisory(in); adv != "" {
		parts = append(parts, adv)
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "; "), block
}

// parseMissAdvisory returns a non-blocking note when scout-report.md has a
// "## Selected Tasks" section holding real content that yielded zero slugs.
//
// missingSlugs cannot see this: zero slugs is its fail-open path, byte-identical
// to a converged cycle that claimed nothing. Surfacing it here is the only
// channel that reaches an operator or agent at all — Gate A's emitted log line —
// and it stays advisory on purpose: a hard block on every zero-slug report would
// false-block every genuine convergence cycle (see SelectedTasksParseMiss).
func (materializationGate) parseMissAdvisory(in core.ReviewInput) string {
	report, ok := readScoutReport(in.Workspace)
	if !ok || !SelectedTasksParseMiss(report) {
		return ""
	}
	// What Gate A actually checked depends on whether the "## Decision Trace"
	// still yielded slugs. On a parse-miss the Selected Tasks body contributes
	// none, so a non-empty union here came from the trace alone and the gate did
	// check those — measured 2026-09-15, 3 of the 59 real cycle-16* fires. Saying
	// it checked NOTHING there is simply false (audit L1, cycle 1685).
	scope := "Gate A therefore checked NOTHING this cycle: a selected task with no eval file " +
		"would pass here and surface at audit instead (cycle-1570)"
	if traced := SelectedSlugs(report); len(traced) > 0 {
		scope = "Gate A therefore checked ONLY the slug(s) the \"## Decision Trace\" supplied (" +
			strings.Join(traced, ", ") + "); anything claimed solely in the drifted section went " +
			"unchecked and would surface at audit instead (cycle-1570)"
	}
	return "ADVISORY (non-blocking): scout-report.md's \"## Selected Tasks\" section has content but " +
		"NO slug parsed out of it — a parse-miss, not convergence. " + scope + ". " +
		"Restate each slug as a \"- **Slug:** <kebab-case>\" bullet inside that section, " +
		"or as a \"## Decision Trace\" entry with finalDecision \"selected\""
}

// ungradedSlugs returns the SELECTED slugs whose eval exists but carries no
// `[code]` grader — the rule the remediation below has always stated. The
// scout owns the eval and is the only phase whose sandbox may write it (the
// builder's denies .evolve/evals on purpose), so this is where a graderless
// eval must fail: at ship it costs a repair round the builder cannot apply.
func (materializationGate) ungradedSlugs(in core.ReviewInput) []string {
	report, ok := readScoutReport(in.Workspace)
	if !ok {
		return nil
	}
	var out []string
	for _, s := range SelectedSlugs(report) {
		p, found := evalFilePath(in.ProjectRoot, in.Workspace, s)
		if !found {
			continue // reported by missingSlugs
		}
		body, err := os.ReadFile(p)
		if err != nil || !strings.Contains(string(body), "[code]") {
			out = append(out, s)
		}
	}
	return out
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
	ungraded := g.ungradedSlugs(in)
	if len(missing) == 0 && len(ungraded) == 0 {
		return ""
	}
	var b strings.Builder
	if len(missing) > 0 {
		paths := make([]string, 0, len(missing))
		for _, s := range missing {
			paths = append(paths, filepath.Join(in.Workspace, ".evolve", "evals", s+".md"))
		}
		b.WriteString("Create the missing eval file(s) — this requires writing NEW files, which is required here:\n  " +
			strings.Join(paths, "\n  ") + "\n")
	}
	if len(ungraded) > 0 {
		paths := make([]string, 0, len(ungraded))
		for _, s := range ungraded {
			p, _ := evalFilePath(in.ProjectRoot, in.Workspace, s)
			paths = append(paths, p)
		}
		b.WriteString("Add at least one `[code]` grader to the eval file(s) that have none:\n  " +
			strings.Join(paths, "\n  ") + "\n")
	}
	b.WriteString("Each must contain at least one `[code]` grader and test BEHAVIOR, not existence. " +
		"Write them at exactly those paths: an eval written only into the cycle worktree is NOT " +
		"visible to this gate. " +
		"Leave scout-report.md's selected slugs unchanged — the report itself is not the defect.")
	return b.String()
}
