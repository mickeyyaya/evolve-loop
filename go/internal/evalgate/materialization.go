package evalgate

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// scoutReportName is the artifact the scout phase writes into the workspace.
const scoutReportName = "scout-report.md"

// materializationGate (Gate A) enforces the scout contract: every slug scout
// SELECTED must have a real .evolve/evals/<slug>.md file. It fires after the
// scout phase, before triage/tdd/build spend tokens (cycle-166).
type materializationGate struct{}

func (materializationGate) name() string                { return "evals-materialized" }
func (materializationGate) appliesTo(phase string) bool { return phase == string(core.PhaseScout) }

func (materializationGate) check(in core.ReviewInput) (string, bool) {
	report, ok := readScoutReport(in.Workspace)
	if !ok {
		return "", false // no report to parse → fail-open
	}
	slugs := SelectedSlugs(report)
	if len(slugs) == 0 {
		return "", false // convergence / parse-empty → fail-open (no claim of work)
	}
	var missing []string
	for _, s := range slugs {
		if _, found := evalFilePath(in.ProjectRoot, in.Workspace, s); !found {
			missing = append(missing, s)
		}
	}
	if len(missing) == 0 {
		return "", false
	}
	return "scout did not materialize evals for selected slug(s): " + strings.Join(missing, ", "), true
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
