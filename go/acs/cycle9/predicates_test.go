//go:build acs

package cycle9

import (
	"path/filepath"
	"testing"

	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC9_001_RegistryRowCountRatchet(t *testing.T) {
	const ceiling = 160
	if got := len(flagregistry.All); got > ceiling {
		t.Errorf("registry row count %d exceeds ratchet ceiling %d.\n"+
			"Net flag additions are blocked; remove flags to lower the count.",
			got, ceiling)
	}
}

func TestC9_003_AllFanoutFlagsAbsentFromRegistry(t *testing.T) {
	fanoutFlags := []string{
		"EVOLVE_FANOUT_AUDITOR",
		"EVOLVE_FANOUT_CACHE_PREFIX",
		"EVOLVE_FANOUT_CACHE_PREFIX_FILE",
		"EVOLVE_FANOUT_CANCEL_ON_CONSENSUS",
		"EVOLVE_FANOUT_CONCURRENCY",
		"EVOLVE_FANOUT_CONSENSUS_K",
		"EVOLVE_FANOUT_CONSENSUS_POLL_S",
		"EVOLVE_FANOUT_CYCLE",
		"EVOLVE_FANOUT_ENABLED",
		"EVOLVE_FANOUT_PARENT_AGENT",
		"EVOLVE_FANOUT_TEST_EXECUTOR",
		"EVOLVE_FANOUT_TIMEOUT",
		"EVOLVE_FANOUT_TRACK_WORKERS",
		"EVOLVE_FANOUT_WORKER_ARTIFACT",
		"EVOLVE_FANOUT_WORKER_NAME",
		"EVOLVE_FANOUT_WORKER_TOKEN",
		"EVOLVE_FANOUT_WORKSPACE",
	}
	for _, name := range fanoutFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — FANOUT flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-9 FANOUT consolidation).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

func TestC9_004_FanoutEnvReadsGoneFromDispatchCmd(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	dispatchFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_fanout_dispatch.go")
	if !acsassert.FileNotContains(t, dispatchFile, `envchain.Int("EVOLVE_FANOUT_CONCURRENCY"`) {
		t.Errorf("RED: cmd_fanout_dispatch.go still reads EVOLVE_FANOUT_CONCURRENCY via envchain.\n"+
			"Cycle-8 anti-gaming rule: config flags must be DELETED from env reads, not hidden.\n"+
			"Builder must remove fanoutEnvConfig() entirely and replace with policy.json-loaded\n"+
			"FanoutPolicy values (parent consensusdispatch.go passes them as CLI flags).\n"+
			"File: %s", dispatchFile)
	}
}

func TestC9_005_FanoutEnvReadsGoneFromSubagentCmd(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	subagentFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_subagent.go")
	if !acsassert.FileNotContains(t, subagentFile, `envchain.IntMin("EVOLVE_FANOUT_CONCURRENCY"`) {
		t.Errorf("RED: cmd_subagent.go still reads EVOLVE_FANOUT_CONCURRENCY via envchain.\n"+
			"Builder must replace the 3 envchain FANOUT reads (lines 504-506) with\n"+
			"policy.json-loaded FanoutPolicy fields (pol.Fanout.Concurrency etc.).\n"+
			"File: %s", subagentFile)
	}
}

func TestC9_006_TestExecutorOsGetenvGoneFromSubagentCmd(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	subagentFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_subagent.go")
	if !acsassert.FileNotContains(t, subagentFile, `os.Getenv("EVOLVE_FANOUT_TEST_EXECUTOR"`) {
		t.Errorf("RED: cmd_subagent.go still reads EVOLVE_FANOUT_TEST_EXECUTOR via os.Getenv.\n"+
			"Builder must convert this to a --test-executor=<cmd> CLI flag in 'evolve subagent run'.\n"+
			"The os.Getenv call at line 410 must be deleted.\n"+
			"File: %s", subagentFile)
	}
}

// TestC9_007_WorkerTokenConstIsSplitForm verifies that the standalone string
// literal "EVOLVE_FANOUT_WORKER_TOKEN" has been removed from recursion.go.
//
// The IPC protocol flag WORKER_TOKEN is allowed to use the split-const form
// (per SSOT §IPC-protocol-allowed): FanoutWorkerTokenEnv = "EVOLVE_" + "FANOUT_WORKER_TOKEN".
// The split form is NOT extracted by the flagreaders guard's Go AST scanner
// (which only extracts BasicLit tokens matching the complete flag name pattern).
//
// // acs-predicate: config-check — verifies structural code change (const form).
//
// RED: recursion.go:33 currently has fanoutWorkerTokenEnv = "EVOLVE_FANOUT_WORKER_TOKEN" (standalone literal).
func TestC9_007_WorkerTokenConstIsSplitForm(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	recursionFile := filepath.Join(root, "go", "internal", "subagent", "recursion.go")
	if !acsassert.FileNotContains(t, recursionFile, `"EVOLVE_FANOUT_WORKER_TOKEN"`) {
		t.Errorf("RED: recursion.go still has standalone 'EVOLVE_FANOUT_WORKER_TOKEN' string literal.\n"+
			"Builder must change to split form:\n"+
			"  const FanoutWorkerTokenEnv = \"EVOLVE_\" + \"FANOUT_WORKER_TOKEN\"\n"+
			"and export it so cmd_subagent.go:351 can use subagent.FanoutWorkerTokenEnv.\n"+
			"This removes it from the flagreaders guard's Go AST standalone-literal scan.\n"+
			"File: %s", recursionFile)
	}
}

func TestC9_008_OrchestratorRefHasNoFanoutAuditor(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	orchRef := filepath.Join(root, "agents", "evolve-orchestrator-reference.md")
	if !acsassert.FileNotContains(t, orchRef, "EVOLVE_FANOUT_AUDITOR") {
		t.Errorf("RED: evolve-orchestrator-reference.md still references EVOLVE_FANOUT_AUDITOR.\n"+
			"Builder must remove lines 271,278 (dead flag — 0 production readers).\n"+
			"File: %s", orchRef)
	}
}

func TestC9_009_ScoutRefHasNoFanoutEnabled(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	scoutRef := filepath.Join(root, "agents", "evolve-scout-reference.md")
	if !acsassert.FileNotContains(t, scoutRef, "EVOLVE_FANOUT_ENABLED") {
		t.Errorf("RED: evolve-scout-reference.md still references EVOLVE_FANOUT_ENABLED.\n"+
			"Builder must remove line 37 (dead flag — 0 production readers).\n"+
			"File: %s", scoutRef)
	}
}

func TestC9_010_ControlFlagsMdHasNoFanoutRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	if !acsassert.FileNotContains(t, controlFlags, "EVOLVE_FANOUT_") {
		t.Errorf("RED: control-flags.md still contains EVOLVE_FANOUT_* entries.\n"+
			"Builder must remove all 17 FANOUT rows from registry_table.go then\n"+
			"regenerate the doc via 'evolve flags generate'.\n"+
			"File: %s", controlFlags)
	}
}

func TestC9_011_FanoutPolicyStructAddedToPolicy(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	policyFile := filepath.Join(root, "go", "internal", "policy", "policy.go")
	if !acsassert.FileContains(t, policyFile, "FanoutPolicy") {
		t.Errorf("RED: internal/policy/policy.go has no FanoutPolicy struct.\n"+
			"Builder must add:\n"+
			"  type FanoutPolicy struct {\n"+
			"    Concurrency int; TimeoutSecs int; CancelOnConsensus bool;\n"+
			"    ConsensusK int; ConsensusPollSecs int; TrackWorkers bool;\n"+
			"    CachePrefixEnabled bool; TestExecutor string;\n"+
			"  }\n"+
			"and a Fanout *FanoutPolicy field to the Policy struct.\n"+
			"File: %s", policyFile)
	}
}

func TestC9_012_RenderJSON_NilReturnsError(t *testing.T) {
	_, err := dossier.RenderJSON(nil)
	if err == nil {
		t.Errorf("RED: dossier.RenderJSON(nil) must return error.\n" +
			"Builder must add nil guard before json.MarshalIndent in render.go.")
	}
}

func TestC9_013_RenderJSON_InvalidDossierReturnsError(t *testing.T) {
	bad := &dossier.Dossier{
		Cycle:        0,
		Goal:         "bad-cycle",
		FinalVerdict: dossier.VerdictPass,
		Phases:       []dossier.PhaseRecord{{Name: "p", Verdict: dossier.VerdictPass}},
	}
	_, err := dossier.RenderJSON(bad)
	if err == nil {
		t.Errorf("RED: dossier.RenderJSON(invalid dossier) must call Validate and return error.\n" +
			"Builder must call d.Validate() at the top of RenderJSON in render.go.")
	}
}

func TestC9_014_RenderJSON_TrailingNewline(t *testing.T) {
	d := &dossier.Dossier{
		Cycle:        1,
		Goal:         "trailing-newline-check",
		FinalVerdict: dossier.VerdictPass,
		Phases:       []dossier.PhaseRecord{{Name: "scout", Verdict: dossier.VerdictPass}},
	}
	out, err := dossier.RenderJSON(d)
	if err != nil {
		t.Fatalf("RenderJSON failed on valid dossier: %v", err)
	}
	if len(out) == 0 || out[len(out)-1] != '\n' {
		last := byte(0)
		if len(out) > 0 {
			last = out[len(out)-1]
		}
		t.Errorf("RED: RenderJSON output must end with '\\n'; got last byte 0x%02x.\n"+
			"Builder must append '\\n' after json.MarshalIndent in render.go.", last)
	}
}

func TestC9_015_RenderMarkdown_InvalidReturnsError(t *testing.T) {
	bad := &dossier.Dossier{
		Cycle:        0,
		Goal:         "bad-markdown",
		FinalVerdict: dossier.VerdictPass,
		Phases:       []dossier.PhaseRecord{{Name: "p", Verdict: dossier.VerdictPass}},
	}
	_, err := dossier.RenderMarkdown(bad)
	if err == nil {
		t.Errorf("RED: dossier.RenderMarkdown(invalid dossier) must call Validate and return error.\n" +
			"Builder must call d.Validate() at the top of RenderMarkdown in render.go.")
	}
}

func TestC9_016_Write_BlankDirReturnsError(t *testing.T) {
	d := &dossier.Dossier{
		Cycle:        1,
		Goal:         "write-blank-dir-check",
		FinalVerdict: dossier.VerdictPass,
		Phases:       []dossier.PhaseRecord{{Name: "scout", Verdict: dossier.VerdictPass}},
	}
	err := dossier.Write(d, "", false)
	if err == nil {
		t.Errorf("RED: dossier.Write(d, \"\", false) must return error.\n" +
			"Builder must check dir == \"\" at top of Write in write.go.")
	}
}

func TestC9_017_Build_BlankWorkspacePathReturnsError(t *testing.T) {
	_, err := dossier.Build(1, dossier.BuildOpts{WorkspacePath: "", Goal: "g"})
	if err == nil {
		t.Errorf("RED: dossier.Build with blank WorkspacePath must return error.\n" +
			"Builder must add WorkspacePath precondition check in build.go.")
	}
}

func TestC9_018_Build_BlankGoalReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := dossier.Build(1, dossier.BuildOpts{WorkspacePath: dir, Goal: ""})
	if err == nil {
		t.Errorf("RED: dossier.Build with empty Goal must return error.\n" +
			"Builder must add Goal precondition check in build.go.")
	}
	_, err = dossier.Build(1, dossier.BuildOpts{WorkspacePath: dir, Goal: "   "})
	if err == nil {
		t.Errorf("RED: dossier.Build with whitespace-only Goal must return error.\n" +
			"Builder must use strings.TrimSpace in the Goal precondition check in build.go.")
	}
}

func TestC9_019_ApplyDefects_BlankOnlyProducesZeroTodos(t *testing.T) {
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   3,
		Verdict: "FAIL",
		Defects: []string{"", "   ", "\t"},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	if len(state.CarryoverTodos) != 0 {
		t.Errorf("RED: blank/whitespace defects must produce 0 todos; got %d.\n"+
			"Builder must add strings.TrimSpace guard in ApplyDefectsAsCarryoverTodos\n"+
			"(failure_learning.go) to skip blank entries.",
			len(state.CarryoverTodos))
	}
}

func TestC9_020_ApplyDefects_MixedSkipsBlanksProducesTwoTodos(t *testing.T) {
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   4,
		Verdict: "FAIL",
		Defects: []string{"", "real defect A", "   ", "real defect B"},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	if got := len(state.CarryoverTodos); got != 2 {
		t.Errorf("RED: mixed blank+real defects must produce exactly 2 todos; got %d.\n"+
			"Builder must skip blank entries in ApplyDefectsAsCarryoverTodos.",
			got)
	}
	for _, want := range []string{"real defect A", "real defect B"} {
		found := false
		for _, todo := range state.CarryoverTodos {
			if strings.Contains(todo.Action, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no todo found for real defect %q", want)
		}
	}
}
