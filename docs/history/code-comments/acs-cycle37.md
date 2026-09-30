# Comment history: `acs/cycle37`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle37/predicates_test.go:3` — above `package cycle37`

```text
// Package cycle37 materializes the cycle-37 acceptance criteria for TWO tasks
// (both in triage ## top_n):
//
//  1. router-config-cluster-37 — migrate 6 EVOLVE_ROUTER_* / EVOLVE_ROUTING_*
//     flags from os.Getenv/applyEnv to policy.RouterPolicy (Configuration Object);
//     delete 3 stale env-path test files; update cmd_router_dispatch_test.go;
//     lower FlagCeiling 89→83; regenerate docs/architecture/control-flags.md.
//     Flags targeted:
//     - EVOLVE_ROUTER_REPLAN      → policy.RouterPolicy.RouterReplan  (default "shadow")
//     - EVOLVE_ROUTING_JUDGE      → policy.RouterPolicy.RoutingJudge  (default false)
//     - EVOLVE_ROUTER_RECON_DIGEST → policy.RouterPolicy.ReconDigest (default false)
//     - EVOLVE_ROUTER_REPLAN_DEPTH → policy.RouterPolicy.ReplanDepth (default 1)
//     - EVOLVE_ROUTER_PLAN_MODEL  → policy.RouterPolicy.PlanModel    (default "")
//     - EVOLVE_ROUTER_PROPOSE_MODEL → policy.RouterPolicy.ProposeModel (default "")
//
//  2. unexplained-outcome-cycle-0 — Route the escaping terminal path (when
//     newCycleRun fails before any phase runs) through outcome recording so
//     cyclehealth.ClassifyOutcome returns FAILED_EXPLAINED instead of
//     FAILED_UNEXPLAINED. Closes the ADR-0044 C1 chokepoint for the cycle-0
//     init-failure path.
//
// AC map (1:1 with triage top_n tasks):
//
//	router-config-cluster-37:
//	  AC1  6 flags absent from Lookup                    → C37_001 (behavioral)
//	  AC2  Registry row count == 83                      → C37_002 (behavioral, count)
//	  AC3  FlagCeiling const == 83                       → C37_003 (config-check, waiver)
//	  AC4  No router env reads in config.go or cmd_cycle.go → C37_004 (config-check, waiver)
//	  AC5  policy.Policy{}.RouterConfig() returns correct defaults → C37_005 (behavioral)
//	  AC6  EVOLVE_WORKTREE_PATH still registered         → C37_006 (behavioral, PRE-EXISTING GREEN)
//	  AC7  flagreaders regression guard green             → manual+checklist (see below)
//	  AC8  control-flags.md has no entries for 6 removed flags → C37_008 (config-check, waiver)
//	  NEG1 3 stale env-path test files absent from disk  → C37_NEG1 (behavioral)
//	  FULL go test ./... exits clean (cycle-35 regression gate) → manual+checklist
//
//	unexplained-outcome-cycle-0:
//	  AC9  ClassifyOutcome on empty workspace returns FAILED_EXPLAINED, not FAILED_UNEXPLAINED → C37_009 (behavioral)
//	  NEG2 ClassifyOutcome on workspace with ship PASS still returns SHIPPED → C37_NEG2 (behavioral, regression)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle37 package;
//	    (b) exit 0 from `cd go && go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) none of the 6 flag name strings appear in any non-test, non-registry Go file:
//	        grep -rn '"EVOLVE_ROUTER_REPLAN"\|"EVOLVE_ROUTING_JUDGE"\|"EVOLVE_ROUTER_RECON_DIGEST"
//	         \|"EVOLVE_ROUTER_REPLAN_DEPTH"\|"EVOLVE_ROUTER_PLAN_MODEL"\|"EVOLVE_ROUTER_PROPOSE_MODEL"'
//	         go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go'
//	         | grep -v 'acs/cycle37' → 0 matches;
//	    (d) all 6 flags had active env reads in config.go:applyEnv or cmd_cycle.go before the sweep;
//	        verify that their env reads (and only their env reads) are removed.
//
//	FULL (go test ./... clean): `cd go && go test ./... -count=1` (not just -tags acs).
//	    Checklist for Auditor:
//	    (a) exit 0 from `cd go && go test ./... -count=1` — the cycle-35 regression gate;
//	    (b) in particular confirm the 3 deleted test files no longer cause compile errors;
//	    (c) cmd_router_dispatch_test.go compiles with updated function signatures.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C37_001 — 6 flags must be ABSENT from Lookup (if Builder misses
//	            any one, Lookup returns ok=true and the test fails immediately).
//	            C37_004 — env-read literals must be ABSENT from their source files
//	            (split-const anti-gaming: removing the registry row without deleting
//	            the os.Getenv / env[...] call is the cycle-8 failure mode).
//	            C37_NEG1 — 3 stale test files must NOT exist on disk (the cycle-35
//	            root cause: these files tested the now-deleted env paths and caused
//	            full-suite red after the migration).
//	Edge/OOD:   C37_002 checks exact count 83; both over-removal (<83) and
//	            under-removal (>83) fail.
//	Lexical:    Lookup / len / FileContains / FileNotContains / RouterConfig() /
//	            os.Stat / ClassifyOutcome — distinct assertion verbs across the suite.
//	Semantic:   registry-absence, row-count, ceiling-const, no-env-reads,
//	            router-config-defaults, worktree-path-present, no-doc-entries,
//	            stale-file-deletion, outcome-classification, outcome-regression —
//	            10 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n tasks
// (router-config-cluster-37 and unexplained-outcome-cycle-0). Deferred tasks
// (per-phase agent config cluster, cycle-audit-cycle-scoped-ci-gap, etc.) get
// zero predicates this cycle.
//
// 1:1 enforcement:
//
//	predicate=9, manual+checklist=2, unverifiable-remove=0 → total AC=11 ✓
//	(AC7=manual+checklist, FULL=manual+checklist; remaining 9 are predicate)
```

### `go/acs/cycle37/predicates_test.go:104` — above `var removedFlags = []string{`

```text
// removedFlags is the canonical list of 6 flags that cycle-37 removes:
//   - EVOLVE_ROUTER_REPLAN:       migrated to policy.RouterPolicy.RouterReplan
//   - EVOLVE_ROUTING_JUDGE:       migrated to policy.RouterPolicy.RoutingJudge
//   - EVOLVE_ROUTER_RECON_DIGEST: migrated to policy.RouterPolicy.ReconDigest
//   - EVOLVE_ROUTER_REPLAN_DEPTH: migrated to policy.RouterPolicy.ReplanDepth
//   - EVOLVE_ROUTER_PLAN_MODEL:   migrated to policy.RouterPolicy.PlanModel
//   - EVOLVE_ROUTER_PROPOSE_MODEL: migrated to policy.RouterPolicy.ProposeModel
```

### `go/acs/cycle37/predicates_test.go:140` — above `func TestC37_004_NoRouterEnvReadsInSourceFiles(t *testing.T) {`

```text
// TestC37_004_NoRouterEnvReadsInSourceFiles verifies that all 6 router-flag
// env-read string literals have been deleted from their source files:
//   - config.go:applyEnv  — reads env["EVOLVE_ROUTER_REPLAN"], env["EVOLVE_ROUTING_JUDGE"],
//     env["EVOLVE_ROUTER_RECON_DIGEST"], env["EVOLVE_ROUTER_REPLAN_DEPTH"]
//   - cmd_cycle.go        — reads os.Getenv("EVOLVE_ROUTER_PLAN_MODEL"),
//     os.Getenv("EVOLVE_ROUTER_PROPOSE_MODEL")
//
// Covers AC4. Anti-gaming (cycle-8 split-const lesson): Builder cannot remove
// the registry rows while leaving the env["..."] or os.Getenv("...") call sites.
// This predicate catches that gap for all 6 flags across both source files.
//
// acs-predicate: config-check
//
// RED: config.go currently reads 4 flags at lines ~530–580; cmd_cycle.go reads
// 2 flags at lines ~574–578. All 6 flag name strings must be absent after migration.
```

### `go/acs/cycle37/predicates_test.go:251` — above `func TestC37_006_WorktreePathStillRegistered(t *testing.T) {`

```text
// TestC37_006_WorktreePathStillRegistered is the no-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the cluster sweep.
// Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface for cycle 37.
//
// Covers AC6 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
```

### `go/acs/cycle37/predicates_test.go:295` — above `func TestC37_NEG1_StaleConfigTestFilesDeleted(t *testing.T) {`

```text
// TestC37_NEG1_StaleConfigTestFilesDeleted verifies that the 3 stale test files
// in go/internal/config/ — which tested the now-deleted env override paths for
// EVOLVE_ROUTER_REPLAN, EVOLVE_ROUTING_JUDGE, and EVOLVE_ROUTER_RECON_DIGEST —
// have been deleted. This was the cycle-35 root cause: the migration was correct
// but these files still referenced the deleted env paths, causing go test ./... to
// fail with compile errors.
//
// Covers NEG1. BEHAVIORAL: os.Stat checks that the files are absent on disk
// (no FileExists helper needed — we expect the error case).
//
// RED: all 3 stale files currently exist on disk.
// GREEN after Builder deletes them in the same diff as the registry row removals.
```

### `go/acs/cycle37/predicates_test.go:326` — above `type phaseTimingEntry struct {`

```text
// ---------------------------------------------------------------------------
// unexplained-outcome-cycle-0 predicates (AC9, NEG2)
// ---------------------------------------------------------------------------
```

### `go/acs/cycle37/predicates_test.go:339` — above `func TestC37_009_EmptyWorkspaceClassifiesExplained(t *testing.T) {`

```text
// TestC37_009_EmptyWorkspaceClassifiesExplained verifies that when the cycle
// workspace has no phase-timing.json and no interaction-summary.json (the
// scenario produced by newCycleRun failing before any phase runs), the outcome
// is NOT FAILED_UNEXPLAINED.
//
// Covers AC9. BEHAVIORAL: directly calls cyclehealth.ClassifyOutcome on a
// temp dir with no files — the exact state the cycle-0 escaping path produces.
// The outcome must be FAILED_EXPLAINED (or any non-UNEXPLAINED outcome) after
// the fix routes the init-failure path through the C1 chokepoint.
//
// Builder can fix this in either of two ways:
//
//	(a) Modify cyclehealth.ClassifyOutcome to treat an empty/absent workspace
//	    as FAILED_EXPLAINED (initialization failed before any phase ran).
//	(b) Make orchestrator.RunCycle write a minimal phase-timing.json entry with
//	    abort_reason before returning the newCycleRun error, so ClassifyOutcome
//	    finds the C1 record and classifies it as FAILED_EXPLAINED.
//
// RED: currently ClassifyOutcome("empty-dir") returns FAILED_UNEXPLAINED
// because no timing file exists and none of the classifier's positive checks
// (ship PASS, salvage, abort_reason, FAIL verdict) match.
```

### `go/acs/cycle37/predicates_test.go:374` — above `func TestC37_NEG2_ShipPassWorkspaceClassifiesShipped(t *testing.T) {`

```text
// TestC37_NEG2_ShipPassWorkspaceClassifiesShipped verifies that a workspace
// containing a phase-timing.json with a ship PASS verdict still classifies as
// SHIPPED. This is the regression guard: the fix for AC9 must not break the
// primary classification path.
//
// Covers NEG2. BEHAVIORAL: constructs a minimal phase-timing.json fixture and
// calls ClassifyOutcome directly.
//
// PRE-EXISTING GREEN: the SHIPPED classification is already correct; this test
// stays GREEN and ensures the cycle-0 fix does not regress it.
```
