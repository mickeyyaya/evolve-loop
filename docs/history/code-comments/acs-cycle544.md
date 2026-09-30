# Comment history: `acs/cycle544`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle544/predicates_test.go:3` — above `package cycle544`

```text
// Package cycle544 materialises the cycle-544 acceptance criteria for the single
// triage-committed (`## top_n`) task: recover-ship-fleet-starvation-observer.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this lane (blocker-solo,
//	Principle 5): recover-ship-fleet-starvation-observer. Every `## deferred`
//	item (report-size-contract-slice1, coverage-ssot-cli-and-gate-wiring, and the
//	rest of the out-of-scope backlog) gets ZERO predicates here — in particular
//	the recovered ciparity.CoverageTestArgs SSOT is deferred Task 3's surface and
//	is deliberately NOT bound by a cycle-544 predicate.
//
// FEATURE CONTEXT
//
//	Cycles 542 and 543 both built the L3 leg of the fleet-concurrency-respect
//	architecture — a work-supply-starvation observer that self-files ONE weighted
//	inbox todo after K consecutive waves realize fewer lanes than configured for a
//	reason OTHER than a quota/capacity shrink — and both FAILed to ship:
//	  - 542: landed the logic in a NEW leaf package internal/fleethealth, which
//	    escaped the apicover completeness gate (TestApicoverEnforce_* went RED).
//	  - 543: correctly moved it into internal/fleet but shipped a NON-HERMETIC
//	    ACS predicate (TestC543_008) that re-ran the whole real-git ship
//	    integration suite as a side effect and t.Fatalf'd on that foreign suite's
//	    exit code; it flaked red under contention and the audit kernel's
//	    Classify() correctly overrode the narrated PASS to FAIL (red_count>0).
//	This cycle recovers the additive, twice-audited diff and replaces the
//	non-hermetic predicate with the hermetic, in-process contract tests below.
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it CALLS the real fleet.StarvationTracker / WaveObservation / BuildStarvationItem
// / WriteTo functions and asserts on the returned value, streak, or written
// artifact. None shells out to a foreign heavyweight suite (that is exactly the
// cycle-543 defect C544_003 guards against). The two file/dir checks (C544_002
// dir-absence, C544_003 self-hermeticity) carry an explicit config-check waiver.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C544_001 StarvationTracker fires on the K-th consecutive starved
//     wave, resets after firing, and a recovered wave resets the streak.
//   - Negative : C544_005 a QUOTA-SHRUNK under-utilized wave is NEVER starvation,
//     no matter how many consecutive such waves occur (the strongest anti-no-op:
//     a naive realized<desired impl that ignores QuotaShrunk FAILS here).
//   - E2E      : C544_004 the `case ran:` side effect — observe→build→WriteTo —
//     writes exactly ONE cause-stable inbox todo, weight clamped up to the floor.
//   - Regression (structural): C544_002 the observer stays inside the enforced
//     internal/fleet package, no internal/fleethealth leaf (cycle-542 anti-regression).
//   - Regression (meta): C544_003 no cycle-544 predicate shells a foreign real-git
//     integration suite (cycle-543 anti-regression).
//
// AC-6 (touched packages build/vet/race clean, repo-wide CI parity) and AC-7
// (guardcmd/opscmd coverage >=80%) are dispositioned manual+checklist (Auditor),
// NOT predicates: a whole-module `go build ./...` / `go vet` / `go test -race` /
// `go test -cover` nested inside an ACS `go test` is the very heavyweight-suite-
// in-a-predicate smell this cycle exists to eliminate, and those checks are the
// audit CI-parity gate's (ADR-0069) and coverage-gate persona's job. See
// test-report.md's Coverage Map + Handoff checklist for the exact commands.
```

### `go/acs/cycle544/predicates_test.go:104` — above `func TestC544_002_ObserverInEnforcedFleetPkg_NoNewLeaf(t *testing.T) {`

```text
// C544_002 — AC-2 (structural anti-regression, config-check). The observer must
// extend the already-apicover-enforced internal/fleet package rather than a new
// leaf: cycle 542 built the identical logic as internal/fleethealth and the
// apicover completeness gate correctly FAILed it. Load-bearing assertions:
// (1) no internal/fleethealth directory exists, and (2) the observer symbol is
// reachable FROM package fleet (the compile-time fleet.StarvationTracker{}
// reference below proves placement — a fleethealth-housed impl would not
// satisfy this import). This is a package-placement invariant, not production
// behaviour, hence the config-check waiver.
//
// acs-predicate: config-check
```

### `go/acs/cycle544/predicates_test.go:130` — above `func TestC544_003_NoPredicateShellsForeignIntegrationSuite(t *testing.T) {`

```text
// C544_003 — AC-3 (meta anti-regression, config-check). The cycle-543 ship
// failure was a predicate that shelled out to a FOREIGN real-git integration
// suite and t.Fatalf'd on its exit code (non-hermetic; flaked under contention).
// This guard reads THIS predicate file and asserts it never references that
// foreign suite path nor builds a nested coverage-profile go-test invocation.
// Both needles are reconstructed from fragments so the guard never matches its
// OWN source text. This is an invariant on the test artifact itself (not
// production behaviour), hence the config-check waiver.
//
// acs-predicate: config-check
```

### `go/acs/cycle544/predicates_test.go:213` — above `func TestC544_005_QuotaShrunkWaveNeverStarves(t *testing.T) {`

```text
// C544_005 — AC-5 (negative / anti-no-op). A quota/capacity shrink that reduces
// realized lanes below configured is NEVER work-supply starvation, no matter how
// many consecutive such waves occur — QuotaShrunk gates the whole detector. An
// implementation that fires on realized<desired while ignoring QuotaShrunk (a
// no-op that "detects starvation" by counting under-utilization) FAILS here.
//
// This quota-shrink rule is superseded by cycle 1782 (starvation-compares-sized-width,
// design doc §7.7 W4): a quota shrink explains only the lanes it removed, so
// WaveObservation.Starved now compares RealizedLanes with the sized width
// (SizedLanes); waves 19 and 20 (1 and 0 of 2 sized lanes) were wrongly counted
// not-starved under this rule. The assertion below still passes through the
// legacy DesiredLanes/QuotaShrunk fallback used only when SizedLanes is unset.
```
