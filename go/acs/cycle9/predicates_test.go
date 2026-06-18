//go:build acs

// Package cycle9 materializes the cycle-9 acceptance criteria for the
// committed top_n task:
//
//	consolidate-fanout-cluster — consolidate all 17 EVOLVE_FANOUT_* env flags
//	into policy.json-backed FanoutPolicy struct + CLI flags for subprocess config;
//	registry count 258 → 241.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	consolidate-fanout-cluster:
//	  AC1      Registry row count == 241                              → C9_001 (behavioral, exact count)
//	  AC2+NEG1 FlagCeiling const == 241                              → C9_002 (config-check, waiver)
//	  AC3+NEG2 All 17 FANOUT flags absent from Lookup                → C9_003 (behavioral, absence)
//	  AC4(a)   fanoutEnvConfig envchain reads gone from dispatch cmd  → C9_004 (config-check, waiver)
//	  AC4(b)   envchain FANOUT reads gone from subagent cmd           → C9_005 (config-check, waiver)
//	  AC4(c)   os.Getenv FANOUT_TEST_EXECUTOR gone from subagent cmd  → C9_006 (config-check, waiver)
//	  NEG3     WORKER_TOKEN const changed to split form (no literal)  → C9_007 (config-check, waiver)
//	  AC9(a)   EVOLVE_FANOUT_AUDITOR absent from orchestrator ref doc → C9_008 (config-check, waiver)
//	  AC9(b)   EVOLVE_FANOUT_ENABLED absent from scout ref doc        → C9_009 (config-check, waiver)
//	  EDGE1    control-flags.md has no EVOLVE_FANOUT_* rows           → C9_010 (config-check, waiver)
//	  EDGE3    FanoutPolicy struct present in internal/policy/policy.go → C9_011 (config-check, waiver)
//
// ACs with manual+checklist disposition (enforced by CI, no cycle predicate needed):
//
//	AC5  (flagregistry tests pass): TestAll_SortedByName + TestRegistry_FlagCeiling in CI
//	AC6  (full suite 0 FAIL): CI pipeline
//	AC8  (flagreaders guard passes): CI acs lane — go test -tags acs ./acs/regression/flagreaders/...
//	AC10 (registry sorted): TestAll_SortedByName in normal CI run
//
// Removed ACs:
//
//	AC7 (ACS cycle9 predicates pass): self-referential — unverifiable-remove
//	EDGE2 (.apicover-enforce has cycle9): pre-existing GREEN (TDD adds ./acs/cycle9/ during RED phase)
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (consolidate-fanout-cluster). Deferred tasks (per-phase agent config cluster,
// BRIDGE/OBSERVER/etc.) get zero predicates this cycle.
package cycle9

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC9_001_RegistryRowCountIs241 verifies that after removing all 17
// EVOLVE_FANOUT_* rows the total registry count is exactly 241.
//
// BEHAVIORAL: calls flagregistry.All directly (the production SSOT slice).
// No source-file grepping; a magic-string patch cannot satisfy this.
//
// RED: len(flagregistry.All) is currently 258, which is 17 rows above 241.
func TestC9_001_RegistryRowCountIs241(t *testing.T) {
	const want = 241
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove all 17 EVOLVE_FANOUT_* rows from registry_table.go.\n"+
			"Both over-removal (< 241) and under-removal (> 241) fail.\n"+
			"Expected: 258 − 17 = 241.",
			got, want)
	}
}

// TestC9_002_FlagCeilingConstIs241 verifies that the FlagCeiling ratchet
// constant in registry_ceiling_test.go has been lowered from 258 to 241
// in the same diff as the row removal.
//
// // acs-predicate: config-check — the constant value is the canonical config
// item; its absence (still 258) directly breaks the ratchet guarantee.
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 258.
func TestC9_002_FlagCeilingConstIs241(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 241") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 241'.\n"+
			"Builder must lower the FlagCeiling constant from 258 to 241 in the same diff\n"+
			"as removing the 17 EVOLVE_FANOUT_* rows (258 − 17 = 241).\n"+
			"File: %s", ceilingFile)
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
