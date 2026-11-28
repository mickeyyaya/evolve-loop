//go:build acs

package cycle2671

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runNamedGCTests(t *testing.T, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go", "test", "-count=1", "-v", "-run", pattern, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Errorf("go test -run %s ./cmd/evolve: exit %d err %v\n%s%s", pattern, code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 10))
		return
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass:\n%s", name, acsassert.OutputTail(stdout, 40))
		}
	}
}

func TestC2671_001_GCReadsTheDispatchTreeRecordsOncePerRun(t *testing.T) {
	runNamedGCTests(t, "TestGCDispatchReadOnce_RecordsCreatedAfterTheFirstPassWaitForTheNextRun")
}

func TestC2671_002_TheTagProofWarningPrintsOncePerTaggedProcessPerRun(t *testing.T) {
	runNamedGCTests(t,
		"TestGCDispatchReadOnce_TheTagProofWarningPrintsOncePerTaggedProcessInEveryMode",
		"TestGCDispatchReadOnce_ACorruptRecordStillFallsBackToTheTagProofOnce",
		"TestGCDispatchReadOnce_ATaggedProcessWithoutARecordWarnsNeverAndStillStops",
	)
}
