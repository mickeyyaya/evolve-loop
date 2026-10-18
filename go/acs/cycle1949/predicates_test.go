//go:build acs

package cycle1949

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runNamed(t *testing.T, pkg, pattern string) (string, int) {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(acsassert.RepoRoot(t), "go"),
		"test", "-count=1", "-v", "-run", pattern, pkg)
	if err != nil && code == 0 {
		t.Fatalf("go test did not run: %v", err)
	}
	return out + stderr, code
}

func requirePass(t *testing.T, pkg, pattern string, tests ...string) {
	t.Helper()
	out, code := runNamed(t, pkg, pattern)
	if code != 0 {
		t.Errorf("RED: go test -run %s %s exit=%d\n%s", pattern, pkg, code, out)
	}
	for _, name := range tests {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not pass\n%s", name, out)
		}
	}
	if strings.Contains(out, "--- SKIP") {
		t.Errorf("a selected test skipped:\n%s", out)
	}
}

func TestC1949_001_TheJSONDecisionComesFromTheParser(t *testing.T) {
	requirePass(t, "./cmd/evolve", "^TestWaveNext_TheJSONDecisionComesFromTheParser$", "TestWaveNext_TheJSONDecisionComesFromTheParser")
}

func TestC1949_002_TheRecordSaveRetriesOnceAndStopsAfterTwoFailures(t *testing.T) {
	requirePass(t, "./cmd/evolve", "^TestWaveNext_TheRecordSave(FailsOnceThenPasses|StopsAfterTwoFailures)$",
		"TestWaveNext_TheRecordSaveFailsOnceThenPasses", "TestWaveNext_TheRecordSaveStopsAfterTwoFailures")
}

func TestC1949_003_TheOldWaveClosesAfterTheNewRecordIsSaved(t *testing.T) {
	requirePass(t, "./cmd/evolve", "^TestWaveNext_TheOldWaveClosesOnlyAfterTheNewRecordIsSaved$", "TestWaveNext_TheOldWaveClosesOnlyAfterTheNewRecordIsSaved")
}

func TestC1949_004_AnUnknownStartTimeWarnsOnce(t *testing.T) {
	requirePass(t, "./cmd/evolve", "^TestWaveOwnsPID_AnUnknownStartTimeWarnsOnce$", "TestWaveOwnsPID_AnUnknownStartTimeWarnsOnce")
}

func TestC1949_005_TheLedgerSealTestBuildsItsOwnFixture(t *testing.T) {
	requirePass(t, "./internal/adapters/ledger", "^TestSeal_RealLedgerCopy$", "TestSeal_RealLedgerCopy")
}
