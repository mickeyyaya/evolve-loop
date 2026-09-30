# Comment history: `acs/cycle38`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle38/predicates_test.go:3` — above `package cycle38`

```text
// Package cycle38 materializes the cycle-38 acceptance criteria for TWO tasks
// (both in triage ## top_n):
//
//  1. router-cli-model-cluster-38 — migrate EVOLVE_ROUTER_CLI and
//     EVOLVE_ROUTER_MODEL from os.Getenv reads in cmd_cycle.go to new CLI/Model
//     string fields on the existing policy.RouterPolicy struct. Remove 2 registry
//     rows. Lower FlagCeiling 83→81.
//
//  2. gc-mode-config-38 — migrate EVOLVE_GC from os.Getenv read in
//     cmd_loop_outcome.go to a new Mode string field on the existing gc.Policy
//     struct. Remove 1 registry row. Lower FlagCeiling 81→80.
//
// Both tasks ship in the same cycle. ACS predicates verify the FINAL state
// (80 flags, ceiling=80) after both tasks complete. Intermediate ceiling-81
// state (Task 1 alone) is not a separate predicate because the audit suite
// runs after Builder finishes both tasks.
//
// AC map (1:1 with triage top_n tasks):
//
//	router-cli-model-cluster-38:
//	  AC1  EVOLVE_ROUTER_CLI absent from registry        → C38_001 (behavioral)
//	  AC2  EVOLVE_ROUTER_MODEL absent from registry      → C38_002 (behavioral)
//	  AC5  No prod env reads in cmd_cycle.go             → C38_005 (config-check, waiver)
//	  AC6  RouterPolicy has CLI+Model fields (compile)   → C38_006 (behavioral, compile-fail RED)
//	  AC7  flagreaders guard green                       → manual+checklist (see below)
//	  AC8  control-flags.md drops both rows              → C38_008 (config-check, waiver)
//	  NEG1 No t.Setenv for old env vars in tests         → C38_NEG1 (config-check, waiver)
//	  FULL go test ./... green                           → manual+checklist (see below)
//	  NOTE AC3 (count==81) and AC4 (ceiling==81) are INTERMEDIATE; superseded by
//	       gc-mode AC2 (count==80) and AC3 (ceiling==80) since both tasks ship together.
//
//	gc-mode-config-38:
//	  AC1  EVOLVE_GC absent from registry               → C38_GC_001 (behavioral)
//	  AC2  len(flagregistry.All) == 80                  → C38_GC_002 (behavioral, count)
//	  AC3  FlagCeiling == 80                            → C38_GC_003 (config-check, waiver)
//	  AC4  No prod EVOLVE_GC reads in cmd_loop_outcome.go → C38_GC_004 (config-check, waiver)
//	  AC5  gc.Policy.Mode field exists (compile)        → C38_GC_005 (behavioral, compile-fail RED)
//	  AC6  Mode off/shadow/enforce recognized           → manual+checklist (see below)
//	  NEG1 Invalid mode logs WARN, skips                → manual+checklist (see below)
//	  AC7  flagreaders guard green                      → manual+checklist (see below)
//	  FULL go test ./... green                          → manual+checklist (see below)
//
// ACs with manual+checklist disposition:
//
//	AC7 / flagreaders guard (both tasks): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) exit 0 from `cd go && go test -tags acs ./acs/regression/flagreaders/...`;
//	    (b) none of the 3 flag name strings appear in any non-test, non-registry Go file:
//	        grep -rn '"EVOLVE_ROUTER_CLI"\|"EVOLVE_ROUTER_MODEL"\|"EVOLVE_GC"' go/ \
//	          --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches.
//
//	AC6 router-cli (RouterConfig wires CLI/Model into resolveRouterDispatch):
//	    Checklist for Auditor:
//	    (a) resolveRouterDispatch in cmd_cycle.go accepts rc policy.RouterPolicy parameter;
//	    (b) when rc.CLI != "", the returned cli equals rc.CLI;
//	    (c) when rc.Model != "", the returned model equals rc.Model;
//	    (d) resolveRouterDispatchFor passes rc to resolveRouterDispatch.
//	    NOTE: see C38_006 for the compile-fail predicate; the behavioral wiring above
//	    is verified by updated unit tests in cmd_cycle_test.go (Builder must update them).
//
//	gc AC6 (Mode off/shadow/enforce recognized by runGCHook):
//	    Checklist for Auditor:
//	    (a) `go test -run TestGCPolicyModeRecognized ./internal/gc/...` exits 0;
//	    (b) switch in runGCHook covers "off", "shadow", "enforce" branches explicitly;
//	    (c) Builder added TestGCPolicyModeDefaultsOff and TestGCPolicyModeRecognized to
//	        go/internal/gc/ (or a new policy_mode_test.go).
//
//	gc NEG1 (Invalid mode logs WARN, skips):
//	    Checklist for Auditor:
//	    (a) `go test -run TestRunGCHook_InvalidModeSkipped ./cmd/evolve/...` exits 0;
//	    (b) cmd_loop_outcome.go's switch default branch writes `[gc] WARN: ...` to stderr
//	        and returns without running gc.Plan; Builder must NOT remove the WARN behavior.
//
//	FULL (go test ./... clean) for both tasks:
//	    Checklist for Auditor:
//	    (a) exit 0 from `cd go && go test ./... -count=1`;
//	    (b) cmd_cycle_test.go compiles with updated resolveRouterDispatch signature;
//	    (c) cmd_router_dispatch_test.go compiles with updated resolveRouterDispatchHealthy
//	        and updated test assertions (no t.Setenv for ROUTER_CLI/MODEL).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C38_001, C38_002, C38_GC_001 — 3 flags must be ABSENT from Lookup
//	            (if Builder misses one, Lookup returns ok=true and the test fails).
//	            C38_005, C38_GC_004 — env-read string literals must be ABSENT from
//	            source files (anti-gaming: removing the registry row without deleting
//	            the os.Getenv call is the cycle-8 split-const failure mode).
//	            C38_NEG1 — old t.Setenv calls must be ABSENT from test files (tests
//	            that call t.Setenv("EVOLVE_ROUTER_CLI",...) test a deleted env path).
//	Edge/OOD:   C38_GC_002 checks EXACT count 80; over-removal (<80) and under-removal
//	            (>80) both fail.
//	Lexical:    Lookup / len / FileNotContains / FileContains / RouterPolicy field access /
//	            gc.Policy field access — distinct assertion verbs across the suite.
//	Semantic:   registry-absence (3), no-env-reads (2), struct-compile (2), no-stale-tests (1),
//	            count (1), ceiling (1), no-doc-entries (2) — 12 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n tasks
// (router-cli-model-cluster-38 and gc-mode-config-38). Deferred tasks (workflow-defaults-cluster,
// legacy-phase-enable-cluster, dynamic-routing-cluster) get zero predicates this cycle.
//
// 1:1 enforcement:
//
//	Task 1: predicate=5, manual+checklist=3 (AC7, FULL, AC6-behavioral), unverifiable-remove=0
//	        → task AC count=8 ✓ (AC1, AC2, AC5, AC6, AC7, AC8, NEG1, FULL;
//	           AC3/AC4 are intermediate and superseded by gc AC2/AC3)
//	Task 2: predicate=4, manual+checklist=5 (AC6, NEG1, AC7, FULL, flagreaders), unverifiable-remove=0
//	        → task AC count=9 ✓ (AC1, AC2, AC3, AC4, AC5, AC6, NEG1, AC7, FULL)
```

### `go/acs/cycle38/predicates_test.go:160` — above `func TestC38_005_NoProdRouterEnvReadsInCmdCycle(t *testing.T) {`

```text
// TestC38_005_NoProdRouterEnvReadsInCmdCycle verifies that the two os.Getenv
// string literals for EVOLVE_ROUTER_CLI and EVOLVE_ROUTER_MODEL have been deleted
// from cmd_cycle.go.
//
// Covers AC5. Anti-gaming (cycle-8 split-const lesson): Builder cannot remove
// the registry rows while leaving the os.Getenv("EVOLVE_ROUTER_CLI") call sites.
// This predicate catches that gap.
//
// acs-predicate: config-check
//
// RED: cmd_cycle.go currently contains os.Getenv("EVOLVE_ROUTER_CLI") at line
// 639 and os.Getenv("EVOLVE_ROUTER_MODEL") at line 642. Both string literals
// (with surrounding double quotes) must be absent after migration.
```

### `go/acs/cycle38/predicates_test.go:320` — above `func TestC38_GC_004_NoProdGCEnvReadInCmdLoopOutcome(t *testing.T) {`

```text
// TestC38_GC_004_NoProdGCEnvReadInCmdLoopOutcome verifies that the os.Getenv
// string literal for EVOLVE_GC has been deleted from cmd_loop_outcome.go.
//
// Covers gc-mode AC4. Anti-gaming (cycle-8 split-const lesson): Builder cannot
// remove the registry row while leaving the os.Getenv("EVOLVE_GC") call site.
//
// acs-predicate: config-check
//
// RED: cmd_loop_outcome.go currently contains os.Getenv("EVOLVE_GC") at line 86.
// The quoted string literal "EVOLVE_GC" must be absent after migration.
```
