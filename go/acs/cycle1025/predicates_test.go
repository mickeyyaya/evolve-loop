//go:build acs

package cycle1025

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	inboxmoverPkg = "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	cmdEvolvePkg  = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite test %s did NOT pass in %s "+
				"(missing — feature unlanded, failing, or hidden behind a build tag the "+
				"default suite skips). exit=%d\ncombined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

func TestC1025_001_TaskQuarantineDrainBehaviour(t *testing.T) {
	assertDefaultSuiteTestsPass(t, inboxmoverPkg,
		"TestReleaseWithQuarantine_AtCeilingQuarantines",
		"TestReleaseWithQuarantine_BelowCeilingReleasesAndCounts",
		"TestReleaseWithQuarantine_SystemLevelNeverQuarantines",
		"TestReleaseFromQuarantine_RoundTrips",
		"TestShouldQuarantine_NamesThePredicate",
	)
}

func TestC1025_002_LoopDrainAndCLIWiring(t *testing.T) {
	assertDefaultSuiteTestsPass(t, cmdEvolvePkg,
		"TestIsTaskLevelFailure_AllClassifications",
		"TestRunInbox_DispatchesQuarantine",
		"TestRunInboxQuarantine_ListEmptyPopulatedAndJSON",
		"TestRunInboxQuarantine_ReleaseSuccessAndMissing",
		"TestRunInboxQuarantine_UsageAndUnknown",
	)
}
