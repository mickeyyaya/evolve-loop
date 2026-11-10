//go:build acs

package cycle2249

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTestCore(t *testing.T, names ...string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", "./internal/core")
	if err != nil || code != 0 {
		t.Fatalf("go test ./internal/core exit=%d: %v\n%s", code, err, acsassert.OutputTail(out+errOut, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass in ./internal/core", name)
		}
	}
}

func TestC2249_001_FailureDossierNamesEachPostmortemFileSorted(t *testing.T) {
	goTestCore(t,
		"TestFailureDossierPostmortems_NamesEachPostmortemFileSorted",
	)
}

func TestC2249_002_NoPostmortemOmitsFieldAndMalformedEntriesAreIgnored(t *testing.T) {
	goTestCore(t,
		"TestFailureDossierPostmortems_NoPostmortemOmitsTheField",
		"TestFailureDossierPostmortems_IgnoresMalformedNamesDirectoriesAndLinks",
	)
}

func TestC2249_003_ProductionDecisionCallersWritePostmortemPathsToTheArtifact(t *testing.T) {
	goTestCore(t,
		"TestFailureDossierPostmortems_AuditFailDecisionWritesThePathsToTheArtifact",
		"TestFailureDossierPostmortems_RetroFloorDecisionWritesThePathsToTheArtifact",
	)
}
