//go:build acs

package cycle24

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var removedFlags = []string{
	"EVOLVE_AUDITOR_CLI",
	"EVOLVE_BUILD_PERMISSION_MODE",
	"EVOLVE_TDD_ENGINEER_CLI",
	"EVOLVE_TDD_ENGINEER_MODEL",
	"EVOLVE_TDD_ENGINEER_PERMISSION_MODE",
}

func TestC24_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-24 per-phase-cli-model-profiles).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

func TestC24_004_NoProductionReaderForRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

	runnerFile := filepath.Join(root, "go", "internal", "phases", "runner", "runner.go")
	if !acsassert.FileNotContains(t, runnerFile, "envchain.Resolve(envchain.PhaseEnvKey") {
		t.Errorf("RED: runner.go still calls envchain.Resolve(envchain.PhaseEnvKey(...)).\n"+
			"Builder must replace the PERMISSION_MODE envchain.Resolve at line ~445\n"+
			"with reqEnv-first then profile.PermissionMode lookup (os.Getenv tier removed).\n"+
			"File: %s", runnerFile)
	}

	coreAdapterFile := filepath.Join(root, "go", "internal", "adapters", "observer", "core_adapter.go")
	count, err := acsassert.CountInGoFunc(coreAdapterFile, "phaseCLI", "a.envGet")
	if err != nil {
		t.Fatalf("CountInGoFunc(phaseCLI, a.envGet): %v", err)
	}
	if count > 0 {
		t.Errorf("RED: phaseCLI in core_adapter.go calls a.envGet %d time(s).\n"+
			"Builder must remove the a.envGet(k) call from the look closure inside phaseCLI\n"+
			"so only req.Env[k] (reqEnv tier-1) is consulted for per-agent CLI.\n"+
			"File: %s", count, coreAdapterFile)
	}
}

func TestC24_005_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, flag := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 5 per-phase config flag rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

func TestC24_006_WorktreePathStillInRegistry(t *testing.T) {
	const worktreePath = "EVOLVE_WORKTREE_PATH"
	if _, ok := flagregistry.Lookup(worktreePath); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH removed.\n"+
			"Builder MUST NOT remove EVOLVE_WORKTREE_PATH from registry_table.go.\n"+
			"It is a live IPC handoff (agents/evolve-tester.md) pinned by C50_009.\n"+
			"This is the same mistake that killed cycles 17 and 18.",
			worktreePath)
	}
}

func TestC24_008_LlmrouteSkipsEnvForPerAgentCLI(t *testing.T) {
	const (
		sentinelCLI   = "sentinel-cli-c24-008-should-not-be-used"
		sentinelModel = "sentinel-model-c24-008-should-not-be-used"
	)

	t.Setenv("EVOLVE_AUDITOR_CLI", sentinelCLI)
	t.Setenv("EVOLVE_TDD_ENGINEER_MODEL", sentinelModel)

	auditorPlan := llmroute.Resolve(
		"auditor", "audit", "balanced",
		map[string]string{},
		nil,
		nil,
		nil,
	)
	if len(auditorPlan.Candidates) > 0 && auditorPlan.Candidates[0] == sentinelCLI {
		t.Errorf("RED: llmroute.Resolve picked up EVOLVE_AUDITOR_CLI=%q from os.Getenv.\n"+
			"envchain.Resolve(perAgentKey, env, ...) tier-2 is still active in resolvePrimary.\n"+
			"Builder must replace envchain.Resolve(perAgentKey, env, ...) with env[perAgentKey]\n"+
			"(reqEnv-only map lookup) — os.Getenv tier removed for per-agent CLI.\n"+
			"Got Candidates: %v", sentinelCLI, auditorPlan.Candidates)
	}

	tddPlan := llmroute.Resolve(
		"tdd-engineer", "tdd", "balanced",
		map[string]string{},
		nil,
		nil,
		nil,
	)
	if tddPlan.Model == sentinelModel {
		t.Errorf("RED: llmroute.Resolve picked up EVOLVE_TDD_ENGINEER_MODEL=%q from os.Getenv.\n"+
			"envchain.Resolve(PhaseEnvKey(agent, \"MODEL\"), env, ...) tier-2 still active in resolveModel.\n"+
			"Builder must replace envchain.Resolve(PhaseEnvKey(agent, \"MODEL\"), env, ...) with\n"+
			"reqEnv-only lookup in resolveModel — os.Getenv tier removed for per-agent MODEL.\n"+
			"Got Model: %q", sentinelModel, tddPlan.Model)
	}
}

func TestC24_NEG1_ProfileCLIIsHonored(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux"}
	plan := llmroute.Resolve(
		"auditor", "audit", "balanced",
		map[string]string{},
		prof,
		nil,
		nil,
	)
	if len(plan.Candidates) == 0 || plan.Candidates[0] != "codex-tmux" {
		t.Errorf("RED: llmroute.Resolve did not honor profile.CLI=%q — got Candidates=%v.\n"+
			"Profile tier-3 must remain operative after the os.Getenv tier-2 removal.\n"+
			"Removing os.Getenv must NOT degrade the profile → CLI routing path.",
			"codex-tmux", plan.Candidates)
	}
	if plan.PrimarySource != "profile.auditor.cli" {
		t.Errorf("RED: expected PrimarySource=%q, got %q.\n"+
			"The source label must reflect profile provenance after the tier-2 removal.",
			"profile.auditor.cli", plan.PrimarySource)
	}
}

func TestC24_NEG2_RuntimeReferenceHasNoRemovedFlagsDoc(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	runtimeRef := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	for _, flag := range removedFlags {
		if !acsassert.FileNotContains(t, runtimeRef, flag) {
			t.Errorf("RED: runtime-reference.md contains %q — should have no operator-facing\n"+
				"documentation for this removed flag (it was StatusInternal, never an operator dial).\n"+
				"File: %s", flag, runtimeRef)
		}
	}
}
