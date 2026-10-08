package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestContractEscalation_PicksFromTheTable(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeRawProfile(t, root, "builder", map[string]any{
		"name": "builder", "cli": "codex-tmux", "cli_fallback": []string{"claude-tmux"}, "allowed_clis": []string{"claude", "codex", "agy"},
	})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles")),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cr := &cycleRun{o: routedOrchestrator(r), req: CycleRequest{ProjectRoot: root}}
	if got := cr.contractDispatchCLI(PhaseBuild, ""); got != "agy-tmux" {
		t.Fatalf("the deliverable came from the table's primary, got %q", got)
	}
	esc, ok := cr.contractEscalationCLI(PhaseBuild, "")
	if !ok || esc != "claude-tmux" {
		t.Fatalf("the escalation steps to the table's next family, never the profile's codex: %q %v", esc, ok)
	}
}

func TestContractEscalation_NeverLeavesTheAllowedSet(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeRawProfile(t, root, "builder", map[string]any{"name": "builder", "cli": "claude-tmux", "allowed_clis": []string{"claude"}})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"claude"}}
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles")),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cr := &cycleRun{o: routedOrchestrator(r), req: CycleRequest{ProjectRoot: root}}
	if esc, ok := cr.contractEscalationCLI(PhaseBuild, ""); ok {
		t.Fatalf("a claude-only agent has no other family to escalate to: %q", esc)
	}
}

func TestProfileForModelRouting_AMintedPhaseReadsItsOwnProfile(t *testing.T) {
	root := t.TempDir()
	writeRawProfile(t, root, "adversarial-review", map[string]any{
		"name": "adversarial-review", "cli": "claude-tmux", "model_tier_envelope": map[string]any{"min": "deep", "max": "deep"},
	})
	o := &Orchestrator{}
	prof := o.profileForModelRouting(root, "adversarial-review")
	if prof == nil || prof.ModelTierEnvelope == nil || prof.ModelTierEnvelope.Max != "deep" {
		t.Fatalf("a phase outside the built-in table resolves its own <phase>.json, closing the clamp gap: %+v", prof)
	}
	if built := o.profileForModelRouting(root, string(PhaseShip)); built != nil {
		t.Fatalf("a phase with no profile stays nil-safe: %+v", built)
	}
}

func routedOrchestrator(r *cliroute.Router) *Orchestrator {
	o := &Orchestrator{}
	WithCLIRouter(r)(o)
	return o
}

func TestOrchestrator_CLIRouterWiredReportsTheInjectedRouter(t *testing.T) {
	if (&Orchestrator{}).CLIRouterWired() {
		t.Fatal("no router injected, none wired")
	}
	r, _, err := cliroute.Build(cliroute.Setup{Profiles: cliroute.SingleProfile{}})
	if err != nil {
		t.Fatal(err)
	}
	if !routedOrchestrator(r).CLIRouterWired() {
		t.Fatal("WithCLIRouter wires the router")
	}
}

func TestContractEscalation_ARefusedRouteIsLoudOnBothPaths(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeRawProfile(t, root, "builder", map[string]any{"name": "builder", "cli": "codex-tmux", "allowed_clis": []string{"claude", "codex", "agy"}})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles")),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cr := &cycleRun{o: routedOrchestrator(r), req: CycleRequest{ProjectRoot: root}, envSnap: map[string]string{"EVOLVE_BUILDER_CLI": "codex-tmux"}}
	var got string
	dispatch := captureStderr(t, func() { got = cr.contractDispatchCLI(PhaseBuild, "") })
	if got != universalContractFallbackCLI || !strings.Contains(dispatch, "phase build: contract escalation has no route") || !strings.Contains(dispatch, "outside the allowed set") {
		t.Fatalf("the dispatch-CLI lookup names its refusal instead of silently falling back: cli=%q stderr=%q", got, dispatch)
	}
	escalate := captureStderr(t, func() { _, _ = cr.contractEscalationCLI(PhaseBuild, "agy-tmux") })
	if !strings.Contains(escalate, "phase build: contract escalation has no route") {
		t.Fatalf("the escalation names the same refusal the same way: %q", escalate)
	}
}

func TestContractEscalation_ADeclaredStopNeverEscalatesToTheUniversalFallback(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeRawProfile(t, root, "builder", map[string]any{"name": "builder", "cli": "agy-tmux", "allowed_clis": []string{"claude", "agy"}})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy"}, AfterChain: "stop"}
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles")),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cr := &cycleRun{o: routedOrchestrator(r), req: CycleRequest{ProjectRoot: root}}
	if esc, ok := cr.contractEscalationCLI(PhaseBuild, ""); ok {
		t.Fatalf("after_chain stop leaves the builder agy alone; the legacy claude fallback must not override the table: %q", esc)
	}
}

func TestContractEscalation_ADeclaredTableWithNoRootRouterIsLoudAndNeverEscalates(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeRawProfile(t, root, "builder", map[string]any{"name": "builder", "cli": "codex-tmux", "allowed_clis": []string{"claude", "codex", "agy"}})
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cr := &cycleRun{o: &Orchestrator{}, req: CycleRequest{ProjectRoot: root}}
	var esc string
	var ok bool

	stderr := captureStderr(t, func() { esc, ok = cr.contractEscalationCLI(PhaseBuild, "codex-tmux") })

	if ok || !strings.Contains(stderr, "contract escalation has no route") || !strings.Contains(stderr, "no compiled router") {
		t.Fatalf("an orchestrator with no router must not escalate on its own under a declared table: esc=%q ok=%v stderr=%q", esc, ok, stderr)
	}
}

func TestContractEscalation_ADeepDispatchNamesAndEscalatesOnlyWhatTheCeilingPermits(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeRawProfile(t, root, "builder", map[string]any{"name": "builder", "cli": "agy-tmux", "model_tier_default": "deep"})
	block := policy.CLIRouting{CLIs: []string{"agy", "agy-claude", "claude"}, Default: []string{"agy", "agy-claude", "claude"}, Tiers: map[string]policy.TierRule{"deep": {CLIs: []string{"agy-claude", "claude"}}}}
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles")),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cr := &cycleRun{o: routedOrchestrator(r), req: CycleRequest{ProjectRoot: root}}

	if got := cr.contractDispatchCLI(PhaseBuild, ""); got != "agy-claude-tmux" {
		t.Fatalf("dispatched CLI = %q, want agy-claude-tmux: agy is not permitted at deep", got)
	}
	if esc, ok := cr.contractEscalationCLI(PhaseBuild, ""); !ok || esc != "claude-tmux" {
		t.Fatalf("escalation = %q %v, want claude-tmux: never the forbidden agy", esc, ok)
	}
}
