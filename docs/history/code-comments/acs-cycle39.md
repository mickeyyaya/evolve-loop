# Comment history: `acs/cycle39`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle39/predicates_test.go:3` — above `package cycle39`

```text
// Package cycle39 materializes the cycle-39 acceptance criteria for TWO tasks
// (both in triage ## top_n):
//
//  1. legacyflags-phase-enable-cluster-39 — migrate 6 phase-enable flags
//     (REQUIRE_INTENT, TRIAGE_DISABLE, PLAN_REVIEW, TEST_PHASE_ENABLED,
//     BUILD_PLANNER, SWARM_PLANNER) from the `legacyFlags` env-map in config.go
//     to `WorkflowPolicy.PhaseEnables` in policy.json (config-as-code).
//     Delete the legacyFlags var and its iteration loop. Thread REQUIRE_INTENT
//     via o.workflowConfig.PhaseEnables["intent"] in cyclerun.go. Fix
//     routingtest/bricks.go IntentRequired() to use PhaseEnabled pattern.
//     Remove 6 registry rows. Lower FlagCeiling 80→74 (intermediate).
//
//  2. consensus-audit-config-39 — migrate EVOLVE_CONSENSUS_AUDIT (1 flag)
//     from os.Getenv in cmd_consensus_dispatch.go to
//     WorkflowPolicy.ConsensusAuditEnabled. Remove the redundant IPC write in
//     cmd_loop_args.go:270. Remove 1 registry row. Lower FlagCeiling 74→73
//     (FINAL).
//
// Both tasks ship in the same cycle audit. The FINAL state (73 flags,
// FlagCeiling=73) is what the audit validates; intermediate state (74 after
// Task 1 alone) has no separate predicate — same pattern as cycle-38.
//
// AC map (1:1 with triage top_n tasks):
//
//	legacyflags-phase-enable-cluster-39:
//	  AC1  6 legacy flags absent from Lookup          → C39_001 (behavioral)
//	  AC2  len(All)==74                               → INTERMEDIATE; superseded by
//	                                                     T2 AC2_CA (count=73 FINAL)
//	  AC3  FlagCeiling==74                            → INTERMEDIATE; superseded by
//	                                                     T2 AC3_CA (ceiling=73 FINAL)
//	  AC4  no prod env reads for 6 flags (anti-gaming)→ C39_004 (config-check, waiver)
//	  AC5  legacyFlags var deleted from config.go     → C39_005 (config-check, waiver)
//	  AC6  WorkflowPolicy.PhaseEnables resolves       → C39_006 (behavioral, compile-fail RED)
//	  AC7  WORKTREE_PATH still registered             → C39_007 (behavioral, PRE-EXISTING GREEN)
//	  AC8  flagreaders guard green                    → manual+checklist (see below)
//	  AC9  control-flags.md drops 6 rows              → C39_009 (config-check, waiver)
//	  NEG1 IntentRequired() uses PhaseEnabled pattern → C39_NEG1 (config-check, waiver)
//	  FULL go test ./... green                        → manual+checklist (see below)
//
//	consensus-audit-config-39:
//	  AC1_CA  CONSENSUS_AUDIT absent from Lookup      → C39_CA_001 (behavioral)
//	  AC2_CA  len(All)==73 (FINAL after both tasks)   → C39_CA_002 (behavioral, count)
//	  AC3_CA  FlagCeiling==73 (FINAL)                 → C39_CA_003 (config-check, waiver)
//	  AC4_CA  no prod env reads for CONSENSUS_AUDIT   → C39_CA_004 (config-check, waiver)
//	  AC5_CA  ConsensusAuditEnabled defaults true     → C39_CA_005 (behavioral, compile-fail RED)
//	  NEG1_CA IPC write removed from cmd_loop_args.go → C39_CA_NEG1 (config-check, waiver)
//	  FULL_CA go test ./... green                     → manual+checklist (see below)
//
// ACs with manual+checklist disposition:
//
//	AC8 / AC8_CA (flagreaders guard): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) exit 0 from `cd go && go test -tags acs ./acs/regression/flagreaders/...`;
//	    (b) none of the 7 flag name strings appear in any non-test, non-registry Go file:
//	        grep -rn '"EVOLVE_REQUIRE_INTENT"\|"EVOLVE_TRIAGE_DISABLE"\|"EVOLVE_PLAN_REVIEW"\|
//	          "EVOLVE_TEST_PHASE_ENABLED"\|"EVOLVE_BUILD_PLANNER"\|"EVOLVE_SWARM_PLANNER"\|
//	          "EVOLVE_CONSENSUS_AUDIT"' go/ --include='*.go' |
//	          grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches.
//	    NOTE: bricks.go intentionally NOT excluded — NEG1 predicate (C39_NEG1) already
//	    verifies the env injection is removed from bricks.go.
//
//	FULL / FULL_CA (go test ./... clean):
//	    Checklist for Auditor:
//	    (a) exit 0 from `cd go && go test ./... -count=1`;
//	    (b) no stale env-path test files in go/internal/config/ referencing the 6 deleted flags;
//	    (c) routingtest package compiles with updated IntentRequired() using PhaseEnabled;
//	    (d) cmd_consensus_dispatch.go compiles with policy.Load call replacing os.Getenv.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C39_001 — 6 flags must be ABSENT from Lookup (any miss returns ok=true → fail).
//	            C39_004 — 6 flag string literals must be ABSENT from config.go/cyclerun.go
//	            (anti-gaming: registry row removal without env-read deletion is the cycle-8 pattern).
//	            C39_NEG1 — env injection must be ABSENT from bricks.go.
//	            C39_CA_001 — CONSENSUS_AUDIT must be ABSENT from Lookup.
//	            C39_CA_NEG1 — IPC write must be ABSENT from cmd_loop_args.go.
//	Edge/OOD:   C39_CA_002 checks EXACT count 73; over-removal (<73) and under-removal (>73) fail.
//	Lexical:    Lookup / len / FileNotContains / FileContains / WorkflowPolicy field access /
//	            WorkflowConfig() resolver / policy.Policy{} zero-value — distinct verbs.
//	Semantic:   registry-absence (7 flags), no-env-reads (multi-file anti-gaming), struct-field
//	            existence (2 new API surfaces), worktree-path-guard, no-doc-entries (7 flags),
//	            row-count, ceiling-const, ipc-write-absent — 9 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for committed top_n tasks
// (legacyflags-phase-enable-cluster-39, consensus-audit-config-39). Deferred tasks
// (STRICT_AUDIT, Dynamic Routing cluster, StatusInternal ~34 flags) get zero predicates.
//
// 1:1 enforcement:
//
//	Task 1: predicate=7, manual+checklist=2 (AC8, FULL), unverifiable-remove=0
//	        AC2+AC3 are INTERMEDIATE and superseded by T2 FINAL predicates → not counted separately
//	        → task 1 ACs: AC1(pred), AC2(intermediate/superseded), AC3(intermediate/superseded),
//	          AC4(pred), AC5(pred), AC6(pred), AC7(pred), AC8(manual), AC9(pred), NEG1(pred), FULL(manual)
//	Task 2: predicate=5, manual+checklist=1 (FULL_CA), unverifiable-remove=0
//	        → task 2 ACs: AC1_CA(pred), AC2_CA(pred), AC3_CA(pred), AC4_CA(pred), AC5_CA(pred),
//	          NEG1_CA(pred), FULL_CA(manual)
```

### `go/acs/cycle39/predicates_test.go:110` — above `var legacyRemovedFlags = []string{`

```text
// legacyRemovedFlags is the canonical list of 6 flags that cycle-39 Task 1 removes
// from the legacyFlags map in config.go and from the registry.
```

### `go/acs/cycle39/predicates_test.go:146` — above `func TestC39_004_NoProdLegacyFlagEnvReadsInSource(t *testing.T) {`

```text
// TestC39_004_NoProdLegacyFlagEnvReadsInSource verifies that the 6 legacy flag name
// string literals have been deleted from their production source files:
//   - config.go: the legacyFlags map var (lines 306–324) where each flag name is a
//     map key string literal, AND the for-loop iteration block (lines ~557–568) which
//     reads env[flag] for each flag in the map.
//   - cyclerun.go: the direct req.Env["EVOLVE_REQUIRE_INTENT"] read at line 240 in
//     newCycleRun (the only flag with a direct read outside the map iteration).
//
// Covers AC4 (and AC5 by extension: if the map key strings are gone, the var is deleted).
// Anti-gaming (cycle-8 split-const lesson): removing registry rows without deleting the
// env reads is the split-const hiding pattern.
//
// acs-predicate: config-check
//
// RED: config.go currently contains all 6 flag name literals in legacyFlags map
// (lines 307–323). cyclerun.go contains "EVOLVE_REQUIRE_INTENT" at line 240.
```

### `go/acs/cycle39/predicates_test.go:261` — above `func TestC39_007_WorktreePathStillRegistered(t *testing.T) {`

```text
// TestC39_007_WorktreePathStillRegistered is the no-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the cluster sweep.
// Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface for cycle 39.
//
// Covers AC7 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
```

### `go/acs/cycle39/predicates_test.go:357` — above `func TestC39_CA_004_NoProdConsensusAuditEnvReads(t *testing.T) {`

```text
// TestC39_CA_004_NoProdConsensusAuditEnvReads verifies that the os.Getenv string
// literal for EVOLVE_CONSENSUS_AUDIT has been deleted from cmd_consensus_dispatch.go.
//
// Covers T2 AC4_CA. Anti-gaming (cycle-8 split-const lesson): Builder cannot remove
// the registry row while leaving the os.Getenv("EVOLVE_CONSENSUS_AUDIT") call site.
// cmd_consensus_dispatch.go must load policy.Load(...) instead.
//
// acs-predicate: config-check
//
// RED: cmd_consensus_dispatch.go currently has ConsensusEnvOff: os.Getenv("EVOLVE_CONSENSUS_AUDIT") == "0"
// at line 31. The quoted string literal "EVOLVE_CONSENSUS_AUDIT" must be absent after migration.
```
