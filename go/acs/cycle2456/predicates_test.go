//go:build acs

package cycle2456

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func journeyPasses(t *testing.T, tests ...string) {
	t.Helper()
	goDir := acsassert.GoModuleDir(t)
	pattern := "^(" + strings.Join(tests, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-run", pattern, "./cmd/evolve")
	if code != 0 {
		t.Fatalf("go test -run %s ./cmd/evolve: code=%d err=%v\n%s", pattern, code, err, acsassert.OutputTail(stdout+stderr, 40))
	}
	for _, name := range tests {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass", name)
		}
	}
}

func TestC2456_001_ABatchStartClosesAndSpendsTheRecordOfAPRWhoseChangeIsOnMain(t *testing.T) {
	journeyPasses(t, "TestJourneySettleClosesAndSpendsALandedPRAtTheNextBatchStart")
}

func TestC2456_002_ABatchStartSpendsTheRecordOfAClosedPRWithAPrintedLineAndKeepsTheRecordOfAnOpenPR(t *testing.T) {
	journeyPasses(t, "TestJourneySettleSpendsTheRecordOfAClosedPRWithAPrintedLine")
}

func TestC2456_003_ABatchStartClosesAnOpenBatchPRWhoseMembersAreLandedOrClosedAndKeepsOneWithAnOpenMember(t *testing.T) {
	journeyPasses(t, "TestJourneySettleClosesAnOpenBatchPRWhoseMembersAreLandedOrClosed")
}
