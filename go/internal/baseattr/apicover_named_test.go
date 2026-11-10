package baseattr

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	t.Parallel()
	var (
		_ Key
		_ Reds
		_ Verdict
		_ Runner
		_ Attributor
	)
	if KeysOf(nil).Attributable() || ParseGoTest("").Attributable() || GoTestRunner(nil) == nil || len(FailedTests("")) != 0 || Names(nil) != "" || New(gitexec.Git{}, nil, 0, nil).MainRef != DefaultMainRef {
		t.Error("an empty input is attributable, or the runner is nil")
	}
}
