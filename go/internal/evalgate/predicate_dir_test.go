package evalgate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func TestPredicateGates_ReadTheCyclePackageTheACSSuiteNames(t *testing.T) {
	ws, wt := cycleWorkspace(t, 4242), t.TempDir()
	dir := filepath.Join(wt, "go", filepath.FromSlash(acssuite.CyclePackage(4242)))
	for path, body := range map[string]string{
		filepath.Join(dir, "predicates_test.go"):          deferredCorePredicates,
		filepath.Join(dir, "flaky_test.go"):               flakyPredicateSrc,
		filepath.Join(dir, "unsatisfiable_test.go"):       unsatisfiablePredicateSrc,
		filepath.Join(ws, triagecap.TriageArtifactName()): triageWithDeferredCore,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var logged []string
	rv := NewReviewer(config.StageShadow).(*reviewer)
	rv.logf = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	rv.Review(context.Background(), core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: wt, ProjectRoot: t.TempDir()})
	for gateName, want := range map[string]string{
		"floor-binding":                 "deferred/dropped this cycle: core",
		"flaky-predicate-shape":         "TestC4242_WholeModuleSweep",
		"unsatisfiable-predicate-shape": "TestC4242_RetiredFileGone",
	} {
		if !loggedFor(logged, gateName, want) {
			t.Errorf("%s did not read the predicates acssuite.CyclePackage names (want %q in its line):\n%s", gateName, want, strings.Join(logged, "\n"))
		}
	}
}

func loggedFor(logged []string, gateName, want string) bool {
	for _, line := range logged {
		if strings.HasPrefix(line, "[evalgate] "+gateName+": ") && strings.Contains(line, want) {
			return true
		}
	}
	return false
}
