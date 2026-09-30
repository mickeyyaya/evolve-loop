//go:build acs

package cycle802

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func runCoreTest(t *testing.T, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			name, corePkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("core test %s did not report PASS (missing, renamed, or not matched)", name)
	}
}

func TestC802_001_non_floor_failure_never_overrides_floor_verdict(t *testing.T) {
	runCoreTest(t, "TestNonFloorPhaseFailure_DoesNotOverrideFloorVerdict")
}

func TestC802_002_non_floor_failure_after_fail_audit_stays_fail(t *testing.T) {
	runCoreTest(t, "TestNonFloorPhaseFailure_FailAudit_StaysFail")
}

func TestC802_003_floor_phase_failure_remains_cycle_fatal(t *testing.T) {
	runCoreTest(t, "TestFloorPhaseFailure_RemainsCycleFatal")
}

func TestC802_004_resume_path_non_floor_failure_never_overrides_floor_verdict(t *testing.T) {
	runCoreTest(t, "TestResumeNonFloorPhaseFailure_DoesNotOverrideFloorVerdict")
}

func TestC802_005_contract_exhaustion_non_floor_degrades_to_skipped_warn(t *testing.T) {
	runCoreTest(t, "TestContractExhaustion_NonFloorPhase_DegradesToSkippedWarn")
}

func TestC802_006_dossier_records_skipped_phases(t *testing.T) {
	runCoreTest(t, "TestDossier_RecordsSkippedPhases")
}

// acs-predicate: config-check
func TestC802_007_retro_memo_allow_network_honest(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, profile := range []string{"retrospective.json", "memo.json"} {
		path := filepath.Join(root, ".evolve", "profiles", profile)
		if !acsassert.JSONFieldEquals(t, path, "sandbox.allow_network", true) {
			t.Errorf("%s: sandbox.allow_network must be declared true", profile)
		}
	}
}
