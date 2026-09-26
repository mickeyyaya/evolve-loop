package evalgate

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
)

// qualityGate (Gate B) blocks a selected slug whose eval predicate is a definite
// tautology; a weak predicate is advisory only.
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
		return "", false
	}
	var halts, warns []string
	for _, s := range slugs {
		path, found := evalFilePath(in.ProjectRoot, in.Workspace, s)
		if !found {
			continue
		}
		res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: path})
		if err != nil {
			continue
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
		return "weak eval predicate(s) for slug(s): " + strings.Join(warns, ", "), false
	}
	return "", false
}
