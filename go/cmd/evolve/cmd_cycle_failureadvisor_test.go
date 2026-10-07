package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func profileRouter(t *testing.T, root string, pol policy.Policy) *cliroute.Router {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	r, _, err := cliroute.Build(cliroute.Setup{Policy: pol, Profiles: profiles.NewFromDir(dir),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func TestFailureAdvisor_ComesFromTheTable(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc, _ := json.Marshal(map[string]any{"name": "failure-advisor", "cli": "codex-tmux", "model_tier_default": "deep"})
	if err := os.WriteFile(filepath.Join(dir, "failure-advisor.json"), doc, 0o644); err != nil {
		t.Fatal(err)
	}
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	if got := failureAdvisorCLI(root, profileRouter(t, root, policy.Policy{CLIRouting: &block}), io.Discard); got != "agy-tmux" {
		t.Fatalf("the failure advisor launches the table's primary, not the profile's codex: %q", got)
	}
	if got := failureAdvisorCLI(root, profileRouter(t, root, policy.Policy{}), io.Discard); got != "codex-tmux" {
		t.Fatalf("with no block the profile's cli stays: %q", got)
	}
}

func TestFailureAdvisorOpts_ResolvesProfileCLI(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc, _ := json.Marshal(map[string]any{"name": "failure-advisor", "cli": "codex-tmux", "model_tier_default": "deep"})
	if err := os.WriteFile(filepath.Join(dir, "failure-advisor.json"), doc, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := len(failureAdvisorOpts(root, profileRouter(t, root, policy.Policy{}), io.Discard)); got != 1 {
		t.Fatalf("failureAdvisorOpts = %d options, want 1 (WithFailureAdvisorCLI from the profile)", got)
	}
}

func TestFailureAdvisorOpts_AbsentProfileFailsOpen(t *testing.T) {
	empty := t.TempDir()
	// Point every resolvellm fallback at the empty tree. failureAdvisorOpts pins
	// GitRoot; the cwd fallback would otherwise read the real repo's profile.
	t.Setenv("EVOLVE_PROJECT_ROOT", empty)
	t.Setenv("EVOLVE_PLUGIN_ROOT", empty)
	if got := len(failureAdvisorOpts(empty, profileRouter(t, empty, policy.Policy{}), io.Discard)); got != 0 {
		t.Fatalf("failureAdvisorOpts = %d options on an empty tree, want 0 (compiled default keeps working)", got)
	}
}

func TestFailureAdvisorOpts_AnUnresolvedRouteIsLoudAndKeepsTheCompiledDefault(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", empty)
	t.Setenv("EVOLVE_PLUGIN_ROOT", empty)
	var console strings.Builder

	opts := failureAdvisorOpts(empty, profileRouter(t, empty, policy.Policy{}), &console)

	if len(opts) != 0 {
		t.Fatalf("failureAdvisorOpts = %d options, want 0: an unresolved route keeps the compiled default", len(opts))
	}
	if got := console.String(); !strings.Contains(got, "[cycle] WARN failure advisor has no route") || !strings.Contains(got, "failure-advisor") {
		t.Fatalf("console = %q, want one WARN line naming the failure advisor's unresolved route", got)
	}
}

func TestFailureAdvisorOpts_AResolvedRouteWritesNothing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc, _ := json.Marshal(map[string]any{"name": "failure-advisor", "cli": "codex-tmux", "model_tier_default": "deep"})
	if err := os.WriteFile(filepath.Join(dir, "failure-advisor.json"), doc, 0o644); err != nil {
		t.Fatal(err)
	}
	var console strings.Builder

	opts := failureAdvisorOpts(root, profileRouter(t, root, policy.Policy{}), &console)

	if len(opts) != 1 || console.Len() != 0 {
		t.Fatalf("options = %d, console = %q; want one option and no line", len(opts), console.String())
	}
}
