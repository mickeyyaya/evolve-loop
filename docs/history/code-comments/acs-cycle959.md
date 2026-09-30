# Comment history: `acs/cycle959`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle959/predicates_test.go:3` — above `package cycle959`

```text
// Package cycle959 materializes the cycle-959 acceptance criteria for this fleet
// lane's sole committed task, adr0072-fleet-pool-halt-unwired (scout-report.md;
// inbox adr0072-fleet-halt-unwired, weight 0.97).
//
// PROBLEM: the WAVE dispatch branch of runLoop (cmd_loop.go:533) already halts
// the batch on an ADR-0072 forged verdict via anyLaneHaltedForSystemFailure, but
// the opt-in POOL dispatch branch (cmd_loop.go:490-498,
// policy.fleet.scheduling=="pool") gets the identical []fleet.Result and never
// consults it — a forged verdict under pool scheduling files the escalation
// dossier + P0 but the BATCH DOES NOT STOP, the churn ADR-0072 exists to prevent.
//
// SUT surface the Builder must add to package main (cmd/evolve), WITHOUT
// modifying this file or the RED unit tests in
// cmd_loop_pool_systemfailure_halt_test.go:
//
//	func dispatchHaltDecision(results []fleet.Result) (rc int, stopReason string, halt bool)
//	    // single-sources detection through anyLaneHaltedForSystemFailure (AC4);
//	    // halt-code lane → (systemFailureHaltExitCode, "system_failure_halt", true),
//	    // otherwise (0, "", false).
//	pool branch (cmd_loop.go `case ran:` ~490): after logging the lane count,
//	    `if rc, sr, halt := dispatchHaltDecision(results); halt {
//	        lr.StopReason = sr; lr.emitFatal(...); return rc }` BEFORE its continue.
//
// PREDICATE STYLE (cycle-85 rule): the SUT lives in `package main` (cmd/evolve),
// which is NOT importable, so every predicate here EXERCISES it via subprocess —
// each requires an explicit "--- PASS: <name>" for the named package-main unit
// test (a rename / skip / vacuous run yields no PASS line; exit 0 alone never
// satisfies a predicate — cycle-951 precedent). The build/vet predicates run the
// toolchain over the touched package. No source-grep predicate exists in this file.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE  → C959_001 (halt-code lane STOPS the batch).
//	NEGATIVE  → C959_002 (ordinary FAIL/launch-error lanes must NOT stop it — the
//	            strongest anti-no-op: a decision that halts on any non-zero exit
//	            fails, and would freeze the never-stop retry loop).
//	EDGE      → C959_002 also pins nil/empty results (via the unit test).
//	SEMANTIC  → halt vs continue are two DISTINCT outcomes, asserted separately.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 halt-code lane halts pool batch (rc + StopReason)   → C959_001 (unit, subprocess)
//	AC2 ordinary/empty failures keep never-stop semantics   → C959_002 (unit, subprocess / NEGATIVE+EDGE)
//	AC3 no wave-path regression (shared decision green)     → C959_003 (wave halt tests, subprocess)
//	AC3 module builds / vet clean                           → C959_004 (build) · C959_005 (vet)
//	AC1 pool branch actually wired (call + StopReason + no continue) → manual+checklist (Auditor diff-scope)
//	AC4 no duplicated halt-detection logic                  → manual+checklist (Auditor grep) — see test-report.md
```
