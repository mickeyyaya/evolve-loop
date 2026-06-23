package evalgate

import (
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/evalqualitycheck"
)

// qualityGate (Gate B) enforces that the selected slugs' eval predicates are
// behavioral, not tautological no-ops (cycle-204). It fires after the tdd
// phase, by which point the eval files exist, and reuses the working
// evalqualitycheck classifier (LevelPass/Warn/Halt). A definite tautology
// (LevelHalt) blocks at enforce; a weak predicate (LevelWarn) is advisory only
// (CLAUDE.md item-7's "block persistent WARN after a soak" is left as a TODO).
type qualityGate struct{}

func (qualityGate) name() string                { return "predicate-quality" }
func (qualityGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

func (qualityGate) check(in core.ReviewInput) (string, bool) {
	report, ok := readScoutReport(in.Workspace)
	if !ok {
		return "", false
	}
	slugs := SelectedSlugs(report)
	if len(slugs) == 0 {
		return "", false // convergence / parse-empty → fail-open
	}
	var halts, warns []string
	for _, s := range slugs {
		path, found := evalFilePath(in.ProjectRoot, in.Workspace, s)
		if !found {
			continue // a missing eval is Gate A's concern; fail-open here
		}
		res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: path})
		if err != nil {
			continue // unreadable → fail-open
		}
		switch res.Overall {
		case evalqualitycheck.LevelHalt:
			halts = append(halts, s)
		case evalqualitycheck.LevelWarn:
			warns = append(warns, s)
		}
	}
	if len(halts) > 0 {
		return "tautological (no-op) eval predicate(s) for slug(s): " + strings.Join(halts, ", "), true
	}
	if len(warns) > 0 {
		// Advisory: surfaced but never blocks (TODO item-7: escalate persistent WARN after a soak).
		return "weak eval predicate(s) for slug(s): " + strings.Join(warns, ", "), false
	}
	return "", false
}
