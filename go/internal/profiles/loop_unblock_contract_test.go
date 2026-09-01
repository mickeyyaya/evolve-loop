package profiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoopUnblockProfilesRouteTimeoutPronePhasesToAgy(t *testing.T) {
	loader := NewFromDir(realProfilesDir(t))
	// Two phases have LEFT this set, both for the same reason — agy-tmux does
	// not honor the structured-sentinel sub-contract even under explicit
	// correction, and a phase whose corrections cannot converge either demotes
	// the contract gate or burns the cycle:
	//   - adversarial-review, 2026-07-29: 7/7 contract failures across
	//     corrections demoted the gate enforce→advisory in BOTH batch-18
	//     wave-1 lanes (see TestAdversarialReviewRoutesToClaudeDeep).
	//   - triage, 2026-07-30: emitted a v1 FAIL sentinel with no
	//     schema_version-2 failure block, corrections exhausted, and the CYCLE
	//     failed on the top-priority queue item's lane (batch-21 cycle-1215).
	//     See TestTriageRoutesToCodexForQuotaBalance (2026-09-02 supersede:
	//     that incident was agy-specific; triage now codex for quota balance).
	//   - retrospective, 2026-08-14: operator-directed model-strength reroute —
	//     agy deep resolves Gemini 3.1 Pro, now the weakest deep-tier model in
	//     the fleet (vs opus and gpt-5.6-sol); retro post-mortems are
	//     reasoning-heavy and move to claude/deep (opus). See
	//     TestRetrospectiveRoutesToClaudeDeep.
	// The durable fix LANDED: second consecutive contract-gate block escalates
	// the re-dispatch CLI (internal/core/contract_escalation.go, soft overlay)
	// — a phase-wide reroute is no longer the only remedy, which is what makes
	// the 2026-09-02 triage supersede below safe to carry.
	for _, name := range []string{"router"} {
		t.Run(name, func(t *testing.T) {
			p, err := loader.Get(name)
			if err != nil {
				t.Fatalf("load profile: %v", err)
			}
			if p.CLI != "agy-tmux" {
				t.Fatalf("CLI=%q, want agy-tmux", p.CLI)
			}
			if len(p.CLIFallback) != 1 || p.CLIFallback[0] != "claude-tmux" {
				t.Fatalf("CLIFallback=%v, want [claude-tmux]", p.CLIFallback)
			}
		})
	}
}

// TestRetrospectiveRoutesToCodexSol pins the 2026-08-26 operator-directed
// reroute (supersedes the 2026-08-14 claude/deep pin, whose own comment named
// this move: "gpt-5.6-sol is the alternative once codex returns from quota
// bench" — codex returned, live-verified at 44% dispatch share with zero
// quota halts). Retro post-mortems are the single biggest deep-tier consumer
// (~40% of deep dispatches) and are NOT adversarial-vs-builder work, so they
// lead the deep→sol arrangement: codex/deep (gpt-5.6-sol at the directed
// rung — see effort_defaults_test.go), claude as the explicit fallback
// (universal-fallback rule; agy stays banned from fallback chains).
func TestRetrospectiveRoutesToCodexSol(t *testing.T) {
	loader := NewFromDir(realProfilesDir(t))
	p, err := loader.Get("retrospective")
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if p.CLI != "codex-tmux" {
		t.Fatalf("CLI=%q, want codex-tmux (2026-08-26 deep-tier sol arrangement)", p.CLI)
	}
	if len(p.CLIFallback) != 1 || p.CLIFallback[0] != "claude-tmux" {
		t.Fatalf("CLIFallback=%v, want [claude-tmux]", p.CLIFallback)
	}
	if p.ModelTierEnvelope == nil || p.ModelTierEnvelope.Default != "deep" {
		t.Fatalf("envelope default must stay deep (gpt-5.6-sol): %+v", p.ModelTierEnvelope)
	}
}

// TestAdversarialReviewRoutesToClaudeDeep pins the 2026-07-29 reroute: the
// adversarial reviewer is a review-class phase (auditor / plan-reviewer /
// premise-challenge house pattern — claude at deep) and its deliverable
// contract requires exact section + machine-verdict compliance that agy did
// not honor under correction (cycles 1171/1172, 7/7 blocks, circuit opened).
// Contract-gate CLI-escalation LANDED (internal/core/contract_escalation.go,
// soft overlay): a second consecutive contract block escalates the
// re-dispatch CLI — the belt behind routing primaries by quota rather than
// by format-compliance-only (the 2026-09-02 triage supersede).
func TestAdversarialReviewRoutesToClaudeDeep(t *testing.T) {
	p, err := NewFromDir(realProfilesDir(t)).Get("adversarial-review")
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if p.CLI != "claude-tmux" {
		t.Fatalf("CLI=%q, want claude-tmux (the contract-compliant reviewer)", p.CLI)
	}
	if len(p.CLIFallback) != 0 {
		t.Fatalf("CLIFallback=%v, want [] (agy is banned from fallback; claude is already primary)", p.CLIFallback)
	}
	if p.ModelTierDefault != "deep" {
		t.Fatalf("ModelTierDefault=%q, want deep (adversarial work is opus-class)", p.ModelTierDefault)
	}
}

// TestTriageRoutesToCodexForQuotaBalance pins the 2026-09-02 reroute
// (supersedes the 2026-07-30 claude pin the way TestRetrospectiveRoutesToCodexSol
// superseded its predecessor). The 2026-07-30 evidence was AGY-specific —
// triage-on-agy emitted a v1 FAIL sentinel with no schema_version-2 block and
// the fix promoted the declared fallback (claude) to primary; codex was never
// the offender, and its contract compliance is since live-proven at volume:
// the retro supersede below carries the quantified run (76 codex dispatches,
// 44% share, cycles 1530-1552, zero quota halts), and the v22.21.0 soak
// (cycles 1589-1594) shipped on codex scout/build handoffs with the contract
// gate green throughout. With claude
// the quota-constrained family and triage firing EVERY cycle, the single-
// writer decision phase moves to codex; claude becomes the fallback
// (universal-fallback rule), the tier stays balanced (this is a quota
// change, not a reasoning-budget change), and the contract-gate + correction
// ladder + second-block CLI escalation remain the format-compliance belts.
func TestTriageRoutesToCodexForQuotaBalance(t *testing.T) {
	p, err := NewFromDir(realProfilesDir(t)).Get("triage")
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if p.CLI != "codex-tmux" {
		t.Fatalf("CLI=%q, want codex-tmux (2026-09-02 quota rebalance)", p.CLI)
	}
	if len(p.CLIFallback) != 1 || p.CLIFallback[0] != "claude-tmux" {
		t.Fatalf("CLIFallback=%v, want [claude-tmux] (universal fallback; agy stays banned)", p.CLIFallback)
	}
	if p.ModelTierDefault != "balanced" {
		t.Fatalf("ModelTierDefault=%q, want balanced — the reroute must not silently change the reasoning budget", p.ModelTierDefault)
	}
}

func TestLoopUnblockProfilesAllowScoutWorkspaceEvalMaterialization(t *testing.T) {
	p, err := NewFromDir(realProfilesDir(t)).Get("scout")
	if err != nil {
		t.Fatalf("load scout profile: %v", err)
	}
	mustContainString(t, p.AllowedTools, "Write(.evolve/runs/cycle-*/.evolve/evals/*)")
	if p.Sandbox == nil {
		t.Fatal("scout sandbox missing")
	}
	mustContainString(t, p.Sandbox.WriteSubpaths, ".evolve/runs/cycle-*/.evolve/evals")
}

func TestLoopUnblockProfileAllowsTestAmplificationWorktreeWrites(t *testing.T) {
	p, err := NewFromDir(realProfilesDir(t)).Get("test-amplification")
	if err != nil {
		t.Fatalf("load test-amplification profile: %v", err)
	}
	if p.Sandbox == nil {
		t.Fatal("test-amplification sandbox missing")
	}
	mustContainString(t, p.Sandbox.WriteSubpaths, "{worktree_path}")
}

func TestLoopUnblockScoutPromptRequiresWorkspaceEvalPath(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "agents", "evolve-scout.md"))
	if err != nil {
		t.Fatalf("read scout persona: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"absolute `workspace` path",
		"<workspace>/.evolve/evals/<task-slug>.md",
		"Do NOT write only to the cycle worktree",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("scout persona missing %q", want)
		}
	}
}

func mustContainString(t *testing.T, got []string, want string) {
	t.Helper()
	for _, s := range got {
		if s == want {
			return
		}
	}
	t.Fatalf("%q not found in %v", want, got)
}
