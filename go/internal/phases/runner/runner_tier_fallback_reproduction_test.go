package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// quotaExhaustedBridge always returns bridge exit=85 (ExitUnknownPrompt,
// incl. provider quota-wall escalations — llmroute.go:45-50) regardless of
// which CLI or model is asked to launch, and records every (cli, model) pair
// attempted, in order, so the test can pin exactly what the runner tried.
type quotaExhaustedBridge struct {
	attempts []string // "cli@model", in the order the runner attempted them
}

func (q *quotaExhaustedBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	q.attempts = append(q.attempts, req.CLI+"@"+req.Model)
	return core.BridgeResponse{ExitCode: 85, Stderr: "provider quota exhausted"},
		errors.New("bridge: launch exit=85")
}

func (q *quotaExhaustedBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// writeQuotaExhaustionProfile drops a profile JSON with an explicit
// model_tier_default (writeFallbackProfile in runner_fallback_test.go hardcodes
// "sonnet", which is already the universal floor tier and can never step
// down — this test needs a tier ABOVE the floor so a step-down is
// observable) into a temp .evolve/profiles dir.
func writeQuotaExhaustionProfile(t *testing.T, agentName, primaryCLI, modelTier string, fallback []string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fb := ""
	if len(fallback) > 0 {
		fb = `, "cli_fallback": ["` + strings.Join(fallback, `","`) + `"]`
	}
	body := `{"name":"` + agentName + `","cli":"` + primaryCLI + `","model_tier_default":"` + modelTier + `"` + fb + `}`
	profileBase := strings.TrimPrefix(agentName, "evolve-")
	if err := os.WriteFile(filepath.Join(dir, profileBase+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

// TestRun_QuotaExhaustedAcrossChain_NeverStepsDownTier is the bug-reproduction
// FAIL_TO_PASS pin for cycle-876's wire-tier-fallback-chain task.
//
// scout-report.md's Conclusion and fault-localization-report.md's #1 suspect
// (confidence 0.95, runner.go:467,500-524,545-582) both identify the same
// gap: when the resolved tier is quota-exhausted (exit 85) across every CLI
// in the chain, dispatch must step down ONE policy.TierRank via
// llmroute.TierChain and re-walk the SAME CLI chain at the lower tier before
// giving up (Acceptance Criteria #2/#5, scout-report.md).
//
// This cycle's build (tier_fallback.go: TierChain/DispatchTiered,
// llmroute.go: Plan.Tiers) added the primitives but — per build-report.md's
// own "Design Notes"/"Discovery" sections — never swapped the runner's actual
// production dispatch call site (BaseRunner.Run, runner.go:561-582) from the
// CLI-only llmroute.Dispatch to llmroute.DispatchTiered. That closure still
// captures a single `model := plan.Model` (runner.go:524) by value for every
// attempt and never consults plan.Tiers, so the fix is INERT in production —
// mirroring the c41fa94b→95f3e79f "plumbed upstream, never consumed at the
// call site" failure shape called out in fault-localization-report.md.
//
// A profile with model_tier_default="opus" (TierRank 3) resolves
// plan.Tiers = ["opus","balanced"] (steps down to the universal "balanced"
// floor, tier_fallback.go:TierChain). Once the runner is fixed to dispatch
// via DispatchTiered, an all-85 CLI chain must be walked TWICE — once at
// "opus", once at "balanced" — for a total of 4 attempts. Today only 2
// happen, both at "opus": the runner gives up at the first tier instead of
// stepping down.
func TestRun_QuotaExhaustedAcrossChain_NeverStepsDownTier(t *testing.T) {
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "opus", prompt: "x"}
	qb := &quotaExhaustedBridge{}
	root := writeQuotaExhaustionProfile(t, "evolve-auditor", "codex-tmux", "opus", []string{"claude-tmux"})
	r := New(Options{
		Hooks:   hooks,
		Bridge:  qb,
		Prompts: fakePromptsFS("evolve-auditor", "x"),
	})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("expected the exhausted chain to surface an error, got nil")
	}

	want := []string{"codex-tmux@opus", "claude-tmux@opus", "codex-tmux@balanced", "claude-tmux@balanced"}
	if len(qb.attempts) != len(want) {
		t.Fatalf("BUG (wire-tier-fallback-chain still inert at runner.go): expected the CLI chain to "+
			"re-walk at the stepped-down tier once exhausted at the resolved tier (%d attempts: %v), "+
			"got %d attempt(s): %v — the runner still dispatches via the CLI-only llmroute.Dispatch "+
			"with a static plan.Model, ignoring plan.Tiers/llmroute.DispatchTiered added this cycle "+
			"(build-report.md Discovery #1: \"production dispatch ... still walks Dispatch (CLI-only)\")",
			len(want), want, len(qb.attempts), qb.attempts)
	}
	for i := range want {
		if qb.attempts[i] != want[i] {
			t.Errorf("attempt[%d] = %q, want %q (full=%v)", i, qb.attempts[i], want[i], qb.attempts)
		}
	}
}
