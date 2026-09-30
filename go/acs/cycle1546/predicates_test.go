//go:build acs

package cycle1546

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

func TestC1546_001_SalvageSnapshotHEADNeverBecomesTheNormalizeBase(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_SalvageSnapshotHEADResolvesToFirstNonSalvageAncestor",
		"TestWorktreeReuseBase_SalvagedWorkIsPendingInTheReviewDiffAfterNormalize",
	)
}

func TestC1546_002_StackedSalvageSnapshotsResolvePastAllOfThem(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_StackedSalvageSnapshotsResolveToTheCommitBeneathAll",
	)
}

func TestC1546_003_OrdinaryReusedHEADIsRecordedUnchanged(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_OrdinaryHEADIsRecordedVerbatim",
		"TestWorktreeReuseBase_CommitMentioningSalvageInBodyIsNotTreatedAsASnapshot",
	)
}

func TestC1546_004_UnresolvableAncestorDegradesLoudlyNeverSilently(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_UnresolvableAncestorWarnsLoudlyAndDoesNotDegradeSilently",
	)
}

func TestC1546_005_GuardIsReachedFromTheProductionProvisioningPath(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_ProvisioningPathRecordsGuardedBaseInCycleState",
	)
}
