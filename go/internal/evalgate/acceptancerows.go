package evalgate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func WithScopePathResolver(resolve func(projectRoot, taskID string) string) Option {
	return func(o *options) { o.scopePathFor = resolve }
}

type acceptanceRowsGate struct {
	resolve func(projectRoot, taskID string) string
}

func (acceptanceRowsGate) name() string                { return "acceptance-rows" }
func (acceptanceRowsGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

func (g acceptanceRowsGate) check(in core.ReviewInput) (string, bool) {
	filed := core.ContractCriteria(in.Workspace, in.ProjectRoot, g.resolve)
	if filed == 0 {
		return "", false
	}
	report, err := os.ReadFile(filepath.Join(in.Workspace, phasecontract.ArtifactFilename(string(core.PhaseTDD))))
	if err != nil {
		return "", false
	}
	section, found := acSection(strings.Split(string(report), "\n"))
	if !found {
		return "", false
	}
	table, ok := acTable(section)
	if !ok || len(table.rows) >= filed {
		return "", false
	}
	return fmt.Sprintf("the Task Contract holds %d filed acceptance criteria, but the AC-Materialization table has %d row(s) — the eval never narrows the acceptance: give each criterion one row, a predicate for it, or the `deferred` disposition when triage deferred it in `## deferred_acceptance`", filed, len(table.rows)), true
}
