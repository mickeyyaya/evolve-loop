package profiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoopUnblockProfilesRouteTimeoutPronePhasesToAgy(t *testing.T) {
	loader := NewFromDir(realProfilesDir(t))
	// Only the router stays on agy; the other phases moved off it.
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

func TestRetrospectiveRoutesToCodexDeep(t *testing.T) {
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
		t.Fatalf("envelope default must stay deep (codex deep tier — gpt-5.6-sol per the 2026-09-10 cost directive; gpt-6-astra since 2026-09-09): %+v", p.ModelTierEnvelope)
	}
}

func TestAdversarialReviewRoutesToClaudeDeep(t *testing.T) {
	p, err := NewFromDir(realProfilesDir(t)).Get("adversarial-review")
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if p.CLI != "claude-tmux" {
		t.Fatalf("CLI=%q, want claude-tmux (the contract-compliant reviewer)", p.CLI)
	}
	for _, fb := range p.CLIFallback {
		if !strings.HasPrefix(fb, "claude") {
			t.Fatalf("CLIFallback=%v: an entry outside the claude family (agy is banned from fallback; codex is the builder's family — the floor)", p.CLIFallback)
		}
	}
	if p.ModelTierDefault != "deep" {
		t.Fatalf("ModelTierDefault=%q, want deep (adversarial work is opus-class)", p.ModelTierDefault)
	}
}

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
