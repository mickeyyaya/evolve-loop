package audit

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func classifyThroughProductionIntegrationTierGate(t *testing.T, req core.PhaseRequest) (string, []core.Diagnostic) {
	t.Helper()
	yes := true
	writeACSVerdictShip(t, req.Workspace, 0, &yes)
	h := hooks{integrationTierCheck: integrationTierCheckDefault}
	verdict, diags, _ := h.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	return verdict, diags
}

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

func TestAuditOrchestration_IntegrationTier_DeadlineKill_MarkerFreeDegradesToWarn(t *testing.T) {
	req := tierFixture(t)
	oldBudget := integrationTierTimeout
	integrationTierTimeout = time.Nanosecond
	t.Cleanup(func() { integrationTierTimeout = oldBudget })
	fn, calls, _ := seqRunFunc(t, []struct {
		Code int
		Out  string
	}{{1, "--- FAIL: TestSlowRed (0.00s)\nFAIL\tpkg\t1.0s\n"}, {-1, "partial toolchain chatter, no verdict lines\nsignal: killed\n"}})
	withFakeRunner(t, killedAtDeadline(fn))

	verdict, diags := classifyThroughProductionIntegrationTierGate(t, req)

	if verdict != core.VerdictPASS {
		t.Fatalf("a marker-free deadline kill must degrade to WARN, not fail the verdict; got %q, diags=%v", verdict, diags)
	}
	if !hasDiagContaining(diags, "budget") {
		t.Errorf("the WARN must name the budget so the operator can raise it; diags=%v", diags)
	}
	if *calls != 2 {
		t.Fatalf("want 2 attempts, got %d", *calls)
	}
}

func TestAuditOrchestration_IntegrationTier_DeadlineKill_FlushedOffendersStillFail(t *testing.T) {
	req := tierFixture(t)
	oldBudget := integrationTierTimeout
	integrationTierTimeout = time.Nanosecond
	t.Cleanup(func() { integrationTierTimeout = oldBudget })
	fn, calls, _ := seqRunFunc(t, []struct {
		Code int
		Out  string
	}{{1, "--- FAIL: TestSlowRed (0.00s)\n"}, {-1, "--- FAIL: TestTruncatedButJudged (0.02s)\nFAIL\tpkg\t3.0s\nsignal: killed\n"}})
	withFakeRunner(t, killedAtDeadline(fn))

	verdict, diags := classifyThroughProductionIntegrationTierGate(t, req)

	if verdict != core.VerdictFAIL {
		t.Fatalf("flushed offenders in a deadline-killed retake are real verdicts and must FAIL; got %q, diags=%v", verdict, diags)
	}
	if !hasDiagContaining(diags, "TestTruncatedButJudged") {
		t.Errorf("the FAIL must name the flushed offender; diags=%v", diags)
	}
	if *calls != 2 {
		t.Fatalf("want 2 attempts, got %d", *calls)
	}
}

type doneProbeCtx struct {
	context.Context
	asked chan struct{}
	once  sync.Once
}

func (c *doneProbeCtx) Done() <-chan struct{} {
	c.once.Do(func() { close(c.asked) })
	return c.Context.Done()
}

var markerFreeKillScript = []struct {
	Code int
	Out  string
}{{1, "--- FAIL: TestSlowRed (0.00s)\nFAIL\tpkg\t1.0s\n"}, {-1, "partial toolchain chatter, no verdict lines\nsignal: killed\n"}}

func TestDecideTier_DeadlineHitRequiresSynchronizedCtx(t *testing.T) {
	t.Run("killedAtDeadline returns only after ctx is done", func(t *testing.T) {
		parent, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx := &doneProbeCtx{Context: parent, asked: make(chan struct{})}
		sawErr := make(chan error, 1)
		inner := func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
			sawErr <- ctx.Err()
			return -1, nil
		}
		go func() { _, _ = killedAtDeadline(inner)(ctx, "go", "", nil, nil, nil, io.Discard, io.Discard) }()
		select {
		case err := <-sawErr:
			t.Fatalf("the scripted runner ran before waiting on ctx (ctx.Err()=%v) — the raced shape", err)
		case <-ctx.asked:
		}
		cancel()
		if err := <-sawErr; err == nil {
			t.Fatal("the scripted runner must observe a done ctx")
		}
	})
	t.Run("synchronized runner reaches the deadline arm through the production seam", func(t *testing.T) {
		req := tierFixture(t)
		oldBudget := integrationTierTimeout
		integrationTierTimeout = time.Nanosecond
		t.Cleanup(func() { integrationTierTimeout = oldBudget })
		fn, _, _ := seqRunFunc(t, markerFreeKillScript)
		withFakeRunner(t, killedAtDeadline(fn))

		verdict, diags := classifyThroughProductionIntegrationTierGate(t, req)

		if verdict != core.VerdictPASS || !hasDiagContaining(diags, "budget") {
			t.Fatalf("a synchronized deadline kill must reach the budget WARN; got %q, diags=%v", verdict, diags)
		}
	})
	t.Run("a runner that returns before its deadline never reaches the deadline arm", func(t *testing.T) {
		req := tierFixture(t)
		oldBudget := integrationTierTimeout
		integrationTierTimeout = time.Hour
		t.Cleanup(func() { integrationTierTimeout = oldBudget })
		fn, _, _ := seqRunFunc(t, markerFreeKillScript)
		withFakeRunner(t, fn)

		verdict, diags := classifyThroughProductionIntegrationTierGate(t, req)

		if verdict != core.VerdictFAIL {
			t.Fatalf("an attempt that returned before its deadline is a red retake, not the budget WARN; got %q, diags=%v", verdict, diags)
		}
	})
}
