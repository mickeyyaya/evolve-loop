package profiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoopUnblockProfilesRouteTimeoutPronePhasesToAgy(t *testing.T) {
	loader := NewFromDir(realProfilesDir(t))
	for _, name := range []string{"router", "triage", "retrospective", "adversarial-review"} {
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
