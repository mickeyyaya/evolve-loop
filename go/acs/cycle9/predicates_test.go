//go:build acs

// Package cycle9 materializes the cycle-9 acceptance criteria for two committed
// top_n tasks (current session: dossier ADR-0055 contracts) plus the prior-session
// fanout-consolidation predicates that have been corrected to match current state.
//
// AC map for current top_n tasks (fix-dossier-render-contracts,
// fix-core-apply-defects-blank-filter):
//
//	fix-dossier-render-contracts:
//	  AC-D1  RenderJSON(nil) returns error                     → C9_012 (behavioral)
//	  AC-D2  RenderJSON(invalid) calls Validate, returns error → C9_013 (behavioral)
//	  AC-D3  RenderJSON output ends with '\n'                  → C9_014 (behavioral)
//	  AC-D4  RenderMarkdown(invalid) returns error             → C9_015 (behavioral)
//	  AC-D5  Write(d,"",false) returns error                   → C9_016 (behavioral)
//	  AC-D6  Build({WorkspacePath:""}) returns error           → C9_017 (behavioral)
//	  AC-D7  Build({Goal:""}) returns error                    → C9_018 (behavioral)
//	  AC-D8  All 7 dossier gap tests PASS                      → manual+checklist (CI)
//
//	fix-core-apply-defects-blank-filter:
//	  AC-C1  All-blank defects → 0 todos                       → C9_019 (behavioral)
//	  AC-C2  Mixed blank+real → exactly 2 todos                → C9_020 (behavioral)
//	  AC-C3  Idempotency/deterministic-ID tests still pass     → manual+checklist (CI)
//	  AC-C4  go test ./internal/core/... → PASS               → manual+checklist (CI)
//
//	Removed ACs:
//	  Full suite 0 FAIL: CI pipeline (manual+checklist)
//	  ACS gate PASS: self-referential (unverifiable-remove)
//
// Prior-session fanout predicates (C9_001–C9_011): corrected for current state.
// C9_001 corrected from exact-241 to ratchet ≤160 (further reductions landed post
// the original session). C9_002 corrected from 241→160 (current FlagCeiling).
// C9_003–C9_011 unchanged (all GREEN, fanout consolidation complete).
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

// TestC9_001_RegistryRowCountRatchet verifies the flag registry does not exceed
// the post-fanout-consolidation ratchet ceiling (≤ 160).
//
// BEHAVIORAL: calls flagregistry.All directly (the production SSOT slice).
// Corrected from original exact-count 241: subsequent consolidation cycles
// lowered the registry further to 160; the ratchet enforces ≤160, not ==241.
// Pre-existing GREEN after correction.
func TestC9_001_RegistryRowCountRatchet(t *testing.T) {
	const ceiling = 160
	if got := len(flagregistry.All); got > ceiling {
		t.Errorf("registry row count %d exceeds ratchet ceiling %d.\n"+
			"Net flag additions are blocked; remove flags to lower the count.",
			got, ceiling)
	}
}

// TestC9_003_AllFanoutFlagsAbsentFromRegistry verifies that all 17
// EVOLVE_FANOUT_* flags are no longer registered after Builder removes
// their rows from registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT
// function. A source edit alone cannot satisfy this; the row must be absent
// for Lookup to return ok=false.
//
// RED: all 17 FANOUT rows are currently present; each Lookup returns (flag, true).
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

// TestC9_004_FanoutEnvReadsGoneFromDispatchCmd verifies that the
// fanoutEnvConfig() function's envchain.Int("EVOLVE_FANOUT_CONCURRENCY",...) call
// has been removed from cmd_fanout_dispatch.go.
//
// This is the cycle-8 anti-gaming SUBSTANCE requirement: config env reads must
// be DELETED (not hidden via split-const), replaced by policy.json-loaded
// FanoutPolicy values passed as CLI flags to the fanout-dispatch subprocess.
//
// // acs-predicate: config-check — verifies structural code requirement (no
// standalone config env reads in the fanout dispatch command implementation).
//
// RED: cmd_fanout_dispatch.go:60 currently has envchain.Int("EVOLVE_FANOUT_CONCURRENCY",...).
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

// TestC9_005_FanoutEnvReadsGoneFromSubagentCmd verifies that the
// envchain.IntMin("EVOLVE_FANOUT_CONCURRENCY",...) call has been removed from
// cmd_subagent.go (lines 504-506: CONCURRENCY, CACHE_PREFIX, TRACK_WORKERS).
//
// // acs-predicate: config-check — substance requirement: 3 envchain FANOUT reads
// in subagent must be replaced by policy.json-loaded FanoutPolicy fields.
//
// RED: cmd_subagent.go:504 currently has envchain.IntMin("EVOLVE_FANOUT_CONCURRENCY",...).
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

// TestC9_006_TestExecutorOsGetenvGoneFromSubagentCmd verifies that the
// os.Getenv("EVOLVE_FANOUT_TEST_EXECUTOR") call has been removed from
// cmd_subagent.go (line 410).
//
// // acs-predicate: config-check — TEST_EXECUTOR moves to a --test-executor=<cmd>
// CLI flag in 'evolve subagent run'; the os.Getenv read must be deleted.
//
// RED: cmd_subagent.go:410 currently has os.Getenv("EVOLVE_FANOUT_TEST_EXECUTOR").
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

// TestC9_008_OrchestratorRefHasNoFanoutAuditor verifies that the dead flag
// EVOLVE_FANOUT_AUDITOR has been removed from evolve-orchestrator-reference.md.
//
// FANOUT_AUDITOR has 0 production readers; its only references are in the
// agents/ doc (lines 271, 278). The scout classified it as "Dead" — removal
// is zero-risk.
//
// // acs-predicate: config-check — verifies dead-flag doc cleanup.
//
// RED: evolve-orchestrator-reference.md:271,278 currently reference EVOLVE_FANOUT_AUDITOR.
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

// TestC9_009_ScoutRefHasNoFanoutEnabled verifies that the dead flag
// EVOLVE_FANOUT_ENABLED has been removed from evolve-scout-reference.md.
//
// FANOUT_ENABLED has 0 production readers; its only reference is in the
// agents/ doc (line 37). The scout classified it as "Dead" — removal is
// zero-risk.
//
// // acs-predicate: config-check — verifies dead-flag doc cleanup.
//
// RED: evolve-scout-reference.md:37 currently references EVOLVE_FANOUT_ENABLED.
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

// TestC9_010_ControlFlagsMdHasNoFanoutRows verifies that the generated doc
// docs/architecture/control-flags.md has no EVOLVE_FANOUT_* entries after
// the 17 registry rows are removed and the doc regenerated.
//
// // acs-predicate: config-check — the doc is generated from the flagregistry
// (source of truth); its absence of FANOUT rows follows from AC3 (rows removed)
// plus the regeneration step ('evolve flags generate').
//
// RED: control-flags.md currently has 37 occurrences of EVOLVE_FANOUT_.
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

// TestC9_011_FanoutPolicyStructAddedToPolicy verifies that the FanoutPolicy
// struct has been added to internal/policy/policy.go.
//
// FanoutPolicy is the Configuration Object that replaces the 8 config
// EVOLVE_FANOUT_* env vars (Concurrency, TimeoutSecs, CancelOnConsensus,
// ConsensusK, ConsensusPollSecs, TrackWorkers, CachePrefixEnabled, TestExecutor).
// It is loaded from .evolve/policy.json and injected into fanout callers.
//
// // acs-predicate: config-check — verifies the new config surface exists.
//
// RED: internal/policy/policy.go currently has no FanoutPolicy type.
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

// ── fix-dossier-render-contracts predicates (C9_012–C9_018) ──────────────────
// All 7 are behavioral: they call the actual dossier functions and assert on
// return values. No source-grepping. RED until Builder adds the nil guard,
// Validate calls, trailing-\n append, and blank-dir/blank-goal checks.

// TestC9_012_RenderJSON_NilReturnsError verifies that RenderJSON(nil) returns
// a non-nil error rather than marshaling nil to "null".
//
// BEHAVIORAL: directly calls dossier.RenderJSON with a nil pointer.
// RED: current impl calls json.MarshalIndent(nil) → ("null", nil).
func TestC9_012_RenderJSON_NilReturnsError(t *testing.T) {
	_, err := dossier.RenderJSON(nil)
	if err == nil {
		t.Errorf("RED: dossier.RenderJSON(nil) must return error.\n" +
			"Builder must add nil guard before json.MarshalIndent in render.go.")
	}
}

// TestC9_013_RenderJSON_InvalidDossierReturnsError verifies that RenderJSON
// calls Validate before marshaling and returns an error on invalid input.
//
// BEHAVIORAL: constructs a dossier with cycle=0 (Validate returns error for
// cycle <= 0), calls RenderJSON, asserts error.
// RED: current impl does not call Validate.
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

// TestC9_014_RenderJSON_TrailingNewline verifies that RenderJSON output ends
// with a trailing '\n' (contract: "Returns UTF-8 JSON with a trailing newline").
//
// BEHAVIORAL: calls RenderJSON on a valid dossier and inspects the last byte.
// RED: json.MarshalIndent does not append '\n'; last byte is '}' (0x7d).
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

// TestC9_015_RenderMarkdown_InvalidReturnsError verifies that RenderMarkdown
// calls Validate before executing the template and returns error on invalid input.
//
// BEHAVIORAL: constructs a dossier with cycle=0, calls RenderMarkdown, asserts err.
// RED: current impl calls template.Execute without Validate.
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

// TestC9_016_Write_BlankDirReturnsError verifies that Write(d, "", false) returns
// an error rather than silently writing to the current working directory.
//
// BEHAVIORAL: calls dossier.Write with a blank dir, asserts error.
// RED: current impl calls filepath.Join("", "cycle-N.json") which resolves to
// the cwd; no precondition check exists.
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

// TestC9_017_Build_BlankWorkspacePathReturnsError verifies that Build returns
// an error when BuildOpts.WorkspacePath is empty.
//
// BEHAVIORAL: calls dossier.Build(1, {WorkspacePath:""}) and asserts error.
// RED: current impl skips the WorkspacePath precondition check.
func TestC9_017_Build_BlankWorkspacePathReturnsError(t *testing.T) {
	_, err := dossier.Build(1, dossier.BuildOpts{WorkspacePath: "", Goal: "g"})
	if err == nil {
		t.Errorf("RED: dossier.Build with blank WorkspacePath must return error.\n" +
			"Builder must add WorkspacePath precondition check in build.go.")
	}
}

// TestC9_018_Build_BlankGoalReturnsError verifies that Build returns an error
// when BuildOpts.Goal is empty or whitespace-only.
//
// BEHAVIORAL: calls dossier.Build twice (empty and whitespace-only Goal), asserts error.
// RED: current impl sets d.Goal = opts.Goal without validation.
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

// ── fix-core-apply-defects-blank-filter predicates (C9_019–C9_020) ───────────
// Both are behavioral: they call ApplyDefectsAsCarryoverTodos and assert on
// the resulting CarryoverTodos slice. RED until Builder adds TrimSpace guard.

// TestC9_019_ApplyDefects_BlankOnlyProducesZeroTodos verifies that a defect
// list containing only blank/whitespace strings produces zero carryover todos.
//
// BEHAVIORAL: constructs a FailedRecord with all-blank Defects, calls
// ApplyDefectsAsCarryoverTodos, asserts len(state.CarryoverTodos) == 0.
// RED: current impl iterates without TrimSpace check; blanks produce todos.
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

// TestC9_020_ApplyDefects_MixedSkipsBlanksProducesTwoTodos verifies that a
// mixed list ["", "real A", "   ", "real B"] produces exactly 2 todos.
//
// BEHAVIORAL: constructs a FailedRecord with 2 real + 2 blank Defects, calls
// ApplyDefectsAsCarryoverTodos, asserts exactly 2 todos with real text.
// RED: current impl produces 4 todos (includes the 2 blank entries).
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
	// Verify real defects appear in the todos (behavioral: checks content, not just count).
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
