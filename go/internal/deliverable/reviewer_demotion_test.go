package deliverable

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestReviewer_CircuitBreaker_MarksDemoted(t *testing.T) {
	ws := t.TempDir() // empty → every Verify violates
	pr := t.TempDir()
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)

	for i := 1; i < 3; i++ {
		got := r.Review(context.Background(), reviewInput("build", ws, pr))
		if got.Approve {
			t.Fatalf("block %d: enforce must still reject before the breaker opens", i)
		}
		if got.Demoted {
			t.Errorf("block %d: Demoted must stay false while the gate is still enforcing (%+v)", i, got)
		}
	}

	got := r.Review(context.Background(), reviewInput("build", ws, pr))
	if !got.Approve {
		t.Fatalf("threshold block must demote enforce→advisory; got %+v", got)
	}
	if !got.Demoted {
		t.Error("circuit open returned Approve without Demoted — the demotion is indistinguishable from a compliant deliverable")
	}
	if got.Reason == "" {
		t.Error("a demoting result must carry the violation reason so the operator WARN can name what stopped being enforced")
	}
}

func TestReviewer_ApprovalIsNeverDemoted(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)
	if got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir())); got.Demoted {
		t.Errorf("a compliant deliverable must not be flagged Demoted: %+v", got)
	}
}

func TestReviewer_ShadowApprovalIsNotDemotion(t *testing.T) {
	r := newTestReviewer(config.StageShadow, filepath.Join(t.TempDir(), "breaker.json"), 3)
	if got := r.Review(context.Background(), reviewInput("build", t.TempDir(), t.TempDir())); got.Demoted {
		t.Errorf("shadow stage is observe-only by design, not a demotion: %+v", got)
	}
}

func exemptInput(phase, workspace, projectRoot string) core.ReviewInput {
	in := reviewInput(phase, workspace, projectRoot)
	in.BreakerExempt = true
	return in
}

func TestReviewer_ABreakerExemptPhaseNeverCountsSoTheNextPhaseIsNeverWaived(t *testing.T) {
	bp := filepath.Join(t.TempDir(), "breaker.json")
	r := newTestReviewer(config.StageEnforce, bp, 3)
	empty, pr := t.TempDir(), t.TempDir()

	for i := 1; i <= 4; i++ {
		if got := r.Review(context.Background(), exemptInput("build", empty, pr)); got.Approve || got.Demoted || got.Blocks != 0 || got.Reason == "" {
			t.Fatalf("exempt block %d = %+v, want a plain rejection with its reason that never counts toward the circuit", i, got)
		}
	}

	if n := readBreaker(bp); n != 0 {
		t.Fatalf("breaker = %d after four exempt blocks, want 0", n)
	}
	if got := r.Review(context.Background(), reviewInput("build", empty, pr)); got.Approve || got.Demoted || got.Blocks != 1 {
		t.Errorf("the next phase's block = %+v, want block 1 and enforced: an exempt phase must never hand the next one a contract_gate_demoted waiver", got)
	}
}

func TestReviewer_ABreakerExemptPassNeverResetsTheCircuit(t *testing.T) {
	bp := filepath.Join(t.TempDir(), "breaker.json")
	r := newTestReviewer(config.StageEnforce, bp, 3)
	compliant := t.TempDir()
	writeFile(t, compliant, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	writeBreaker(bp, 2)

	if got := r.Review(context.Background(), exemptInput("build", compliant, t.TempDir())); !got.Approve || got.Demoted {
		t.Fatalf("a compliant exempt deliverable = %+v, want approved", got)
	}

	if n := readBreaker(bp); n != 2 {
		t.Errorf("breaker = %d after an exempt pass, want 2: the exempt phase is invisible to the circuit either way", n)
	}
}

func TestReviewer_ARejectionSaysWhetherTheDeliverableIsAbsent(t *testing.T) {
	malformed := t.TempDir()
	writeFile(t, malformed, "build-report.md", "no sections here\nVerdict: PASS\n")
	blank := t.TempDir()
	writeFile(t, blank, "build-report.md", "  \n")
	for name, tc := range map[string]struct {
		workspace string
		exempt    bool
		absent    bool
	}{
		"missing, exempt":    {t.TempDir(), true, true},
		"empty, exempt":      {blank, true, true},
		"malformed, exempt":  {malformed, true, false},
		"missing, counted":   {t.TempDir(), false, true},
		"malformed, counted": {malformed, false, false},
	} {
		r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)
		in := reviewInput("build", tc.workspace, t.TempDir())
		in.BreakerExempt = tc.exempt

		got := r.Review(context.Background(), in)

		if got.Approve || got.DeliverableAbsent != tc.absent {
			t.Errorf("%s: %+v, want a rejection with DeliverableAbsent=%v: only a missing or empty deliverable is absent", name, got, tc.absent)
		}
	}
}

func TestReviewer_AnExemptBlockIsSignalledWithBreakerCountZero(t *testing.T) {
	r, got := observedReviewer(t, config.StageEnforce)
	in := reviewInput("build", t.TempDir(), t.TempDir())
	in.BreakerExempt = true

	r.Review(context.Background(), in)

	if e := oneGateEvent(t, *got, "GATE_CONTRACT_REJECTED"); e.Fields["blocks"] != "0" {
		t.Errorf("GATE_CONTRACT_REJECTED blocks = %q, want 0: an exempt block is reported and never counted", e.Fields["blocks"])
	}
}

func TestReviewer_AnExemptPhaseBelowEnforceOnlyWouldBlock(t *testing.T) {
	for _, stage := range []config.Stage{config.StageShadow, config.StageAdvisory} {
		r := newTestReviewer(stage, filepath.Join(t.TempDir(), "breaker.json"), 3)
		in := reviewInput("build", t.TempDir(), t.TempDir())
		in.BreakerExempt = true

		if res := r.Review(context.Background(), in); !res.Approve {
			t.Errorf("stage %s: exempt missing deliverable = %+v, want approved as a would-block: the exemption never outranks the gate's stage", stage, res)
		}
	}
}
