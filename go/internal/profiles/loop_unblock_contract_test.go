package profiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoopUnblockProfilesRouteTimeoutPronePhasesToAgy(t *testing.T) {
	loader := NewFromDir(realProfilesDir(t))
	// adversarial-review left this set 2026-07-29: agy ignored the phase's
	// deliverable contract 7/7 times across correction re-dispatches (its own
	// report format, no machine verdict line), which demoted the contract
	// gate enforce→advisory in BOTH batch-18 wave-1 lanes — a gate-weakening
	// outcome. See TestAdversarialReviewRoutesToClaudeDeep below.
	for _, name := range []string{"router", "triage", "retrospective"} {
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

// TestAdversarialReviewRoutesToClaudeDeep pins the 2026-07-29 reroute: the
// adversarial reviewer is a review-class phase (auditor / plan-reviewer /
// premise-challenge house pattern — claude at deep) and its deliverable
// contract requires exact section + machine-verdict compliance that agy did
// not honor under correction (cycles 1171/1172, 7/7 blocks, circuit opened).
// Contract-gate CLI-escalation is queued (contract-block-cli-escalation);
// until it lands, the primary CLI must be the format-compliant one.
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
