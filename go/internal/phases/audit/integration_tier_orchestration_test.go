package audit

// integration_tier_orchestration_test.go — cycle-1554 RED contract for
// `integration-tier-contention-retake-accountability` (inbox
// pipeline-defect-pipeline-blocker, P0).
//
// ciparity_unit_test.go already proves CheckIntegrationTier itself returns
// (nil, flake-error) on red-then-green and (offenders, nil) on red-then-red.
// That is necessary but not sufficient: nothing in the suite wires the
// PRODUCTION integrationTierCheckDefault (the function NewDefaultWithStageCompact
// actually installs at audit.go:816/862, as h.integrationTierCheck) through the
// real hooks.Classify orchestration and asserts on the AUDIT VERDICT the gate
// produces. audit_verdict_conflict_gates_test.go exercises Classify's
// override/no-override wiring, but only via hand-authored offenders(...)/
// cannotRun(...) stand-ins for h.integrationTierCheck — never the real
// red-then-green-retake logic. A regression that broke the seam between
// CheckIntegrationTier's return shape and applyCIGate's (cerr!=nil ⇒ WARN,
// offenders>0 ⇒ FAIL) branching — e.g. a future refactor that stopped mapping
// the flake error into a could-not-run WARN — would pass every existing test
// in this package while silently turning every contention flake into a false
// audit FAIL (or laundering a genuine red-then-red into a WARN). These two
// tests close that gap: same subprocess-level fixtures as ciparity_unit_test.go,
// but driven through the real Classify orchestration.

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// classifyThroughProductionIntegrationTierGate runs the real hooks.Classify
// with the production integrationTierCheckDefault wired exactly as
// NewDefaultWithStageCompact wires it — every other gate left nil so only the
// integration-tier gate's could-not-run/offenders branch can move the
// verdict. The EGPS verdict is written GREEN (writeACSVerdictShip) so none of
// the three EGPS override branches — already covered by
// audit_verdict_conflict_test.go — can fire and confound the result.
func classifyThroughProductionIntegrationTierGate(t *testing.T, req core.PhaseRequest) (string, []core.Diagnostic) {
	t.Helper()
	yes := true
	writeACSVerdictShip(t, req.Workspace, 0, &yes)
	h := hooks{integrationTierCheck: integrationTierCheckDefault}
	verdict, diags, _ := h.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	return verdict, diags
}

// TestAuditOrchestration_IntegrationTier_RedThenGreen_ContentionAbsorbedToWarn —
// a red first attempt that goes GREEN on the serialized clean-env retake must
// surface as a visible warning, not fail the AUDIT ORCHESTRATION verdict (not
// merely CheckIntegrationTier's own return value in isolation).
func TestAuditOrchestration_IntegrationTier_RedThenGreen_ContentionAbsorbedToWarn(t *testing.T) {
	req := tierFixture(t)
	fn, calls, _ := seqRunFunc(t, []struct {
		Code int
		Out  string
	}{{1, "--- FAIL: TestFlaky (0.00s)\nFAIL\tpkg\t1.0s\n"}, {0, "ok\n"}})
	withFakeRunner(t, fn)

	verdict, diags := classifyThroughProductionIntegrationTierGate(t, req)

	if verdict != core.VerdictPASS {
		t.Fatalf("a contention flake (red-then-green serialized retake) must not fail the audit orchestration verdict; got %q, diags=%v", verdict, diags)
	}
	if !hasDiagContaining(diags, "flake") {
		t.Errorf("Classify must surface a visible warning naming the flake; diags=%v", diags)
	}
	if got := conflictDiags(diags); len(got) != 0 {
		t.Errorf("a fail-open (could-not-run) gate outcome must never register as a verdict-conflict record: %v", got)
	}
	if *calls != 2 {
		t.Fatalf("want exactly 2 subprocess attempts (first attempt + serialized retake), got %d", *calls)
	}
}

// TestAuditOrchestration_IntegrationTier_RedThenRed_GenuineFailReachesTheAuditVerdict —
// a red retake is a genuine defect and must reach FAIL through the real
// orchestration, naming the retake's own (truthful) offenders.
func TestAuditOrchestration_IntegrationTier_RedThenRed_GenuineFailReachesTheAuditVerdict(t *testing.T) {
	req := tierFixture(t)
	fn, calls, _ := seqRunFunc(t, []struct {
		Code int
		Out  string
	}{{1, "--- FAIL: TestNoisyFirst (0.00s)\n"}, {1, "--- FAIL: TestGenuine (0.01s)\nFAIL\tpkg\t2.0s\n"}})
	withFakeRunner(t, fn)

	verdict, diags := classifyThroughProductionIntegrationTierGate(t, req)

	if verdict != core.VerdictFAIL {
		t.Fatalf("a genuine red-then-red integration failure must FAIL the audit orchestration verdict; got %q, diags=%v", verdict, diags)
	}
	if !hasDiagContaining(diags, "TestGenuine") {
		t.Errorf("the FAIL diagnostic must name the offenders from the truthful serialized retake; diags=%v", diags)
	}
	if *calls != 2 {
		t.Fatalf("want exactly 2 subprocess attempts, got %d", *calls)
	}
}

// TestAuditOrchestration_IntegrationTier_RetakeInfraFailure_FallsBackNotLaundered —
// when the RETAKE itself cannot run (not merely red), the gate must fall back
// to attempt 1's offenders and still FAIL — infra trouble on the retake must
// never launder a real first-attempt red into a pass. Regresses the "retake
// exec error" fallback path (ciparity.go: `cerr2 != nil`) through the same
// production Classify seam as the two cases above.
func TestAuditOrchestration_IntegrationTier_RetakeInfraFailure_FallsBackNotLaundered(t *testing.T) {
	req := tierFixture(t)
	calls := 0
	fn := func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		calls++
		if calls == 1 {
			_, _ = io.WriteString(so, "--- FAIL: TestFirstAttempt (0.00s)\nFAIL\tpkg\t1.0s\n")
			return 1, nil
		}
		return 0, errors.New("exec: fork/exec go: resource temporarily unavailable")
	}
	withFakeRunner(t, fn)

	verdict, diags := classifyThroughProductionIntegrationTierGate(t, req)

	if verdict != core.VerdictFAIL {
		t.Fatalf("a first-attempt red whose retake could not even run must still FAIL, not be silently absorbed; got %q, diags=%v", verdict, diags)
	}
	if !hasDiagContaining(diags, "TestFirstAttempt") {
		t.Errorf("offenders must fall back to attempt 1 (the retake never produced any); diags=%v", diags)
	}
	if calls != 2 {
		t.Fatalf("want exactly 2 attempts (first + failed retake), got %d", calls)
	}
}
