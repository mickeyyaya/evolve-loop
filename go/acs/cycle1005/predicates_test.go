//go:build acs

package cycle1005

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

func runBridgeTest(t *testing.T, pattern string, wantPass ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, bridgePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, bridgePkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not report PASS (renamed, skipped, or not run):\n%s", name, stdout)
		}
	}
}

func TestC1005_001_tripwire_fires_on_nonclaude_exit0_success(t *testing.T) {
	runBridgeTest(t,
		"^TestRecordTokenUsage_Tripwire_NonClaudeExit0Success_Warns$",
		"TestRecordTokenUsage_Tripwire_NonClaudeExit0Success_Warns")
}

func TestC1005_002_tripwire_stays_silent_on_non_signals(t *testing.T) {
	runBridgeTest(t,
		"^TestRecordTokenUsage_Tripwire_(QuotaAbortShortLaunch|Exit0ShortDuration|ClaudeBaseline|CoveredNonClaude)_Silent$",
		"TestRecordTokenUsage_Tripwire_QuotaAbortShortLaunch_Silent",
		"TestRecordTokenUsage_Tripwire_Exit0ShortDuration_Silent",
		"TestRecordTokenUsage_Tripwire_ClaudeBaseline_Silent",
		"TestRecordTokenUsage_Tripwire_CoveredNonClaude_Silent")
}

func TestC1005_003_tripwire_fail_open_when_cycle_absent(t *testing.T) {
	runBridgeTest(t,
		"^TestRecordTokenUsage_Tripwire_NoCycleInPath_FailOpen$",
		"TestRecordTokenUsage_Tripwire_NoCycleInPath_FailOpen")
}

func TestC1005_004_tripwire_record_field_is_queryable(t *testing.T) {
	runBridgeTest(t,
		"^TestRecordTokenUsage_TripwireRecord_FieldFlips$",
		"TestRecordTokenUsage_TripwireRecord_FieldFlips")
}
