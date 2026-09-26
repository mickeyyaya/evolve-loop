package evalgate

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

var scoutReportName = phasecontract.ArtifactName(string(core.PhaseScout))

// materializationGate (Gate A) requires every slug scout selected to have an
// eval file carrying a [code] grader, before later phases spend tokens.
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
	// Latched before the advisory is appended, so a parse-miss can neither block
	// nor mask a real missing eval.
	block := len(parts) > 0
	if adv := g.parseMissAdvisory(in); adv != "" {
		parts = append(parts, adv)
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "; "), block
}

// parseMissAdvisory returns a non-blocking note when the "## Selected Tasks"
// section has content but yielded no slug, which fail-open would otherwise hide.
func (materializationGate) parseMissAdvisory(in core.ReviewInput) string {
	report, ok := readScoutReport(in.Workspace)
	if !ok || !SelectedTasksParseMiss(report) {
		return ""
	}
	// A non-empty union here came from the Decision Trace alone, and Gate A did check those slugs.
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

// ungradedSlugs returns the selected slugs whose eval exists but has no [code]
// grader. Only the scout may write evals, so a graderless one must fail here.
func (materializationGate) ungradedSlugs(in core.ReviewInput) []string {
	report, ok := readScoutReport(in.Workspace)
	if !ok {
		return nil
	}
	var out []string
	for _, s := range SelectedSlugs(report) {
		p, found := evalFilePath(in.ProjectRoot, in.Workspace, s)
		if !found {
			continue
		}
		body, err := os.ReadFile(p)
		if err != nil || !strings.Contains(string(body), "[code]") {
			out = append(out, s)
		}
	}
	return out
}

// missingSlugs returns the selected slugs with no eval file. check and
// remediation both use it, so they never disagree about which files to create.
func (materializationGate) missingSlugs(in core.ReviewInput) []string {
	report, ok := readScoutReport(in.Workspace)
	if !ok {
		return nil
	}
	slugs := SelectedSlugs(report)
	if len(slugs) == 0 {
		return nil
	}
	var missing []string
	for _, s := range slugs {
		if _, found := evalFilePath(in.ProjectRoot, in.Workspace, s); !found {
			missing = append(missing, s)
		}
	}
	return missing
}

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

// evalFilePath finds slug's eval under projectRoot, then under workspace.
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

// remediation names the exact file per missing or ungraded slug and the grader
// requirement. A missing eval is offered only its workspace path, because the
// sandbox makes projectRoot deny-write.
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
