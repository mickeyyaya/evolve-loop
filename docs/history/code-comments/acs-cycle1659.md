# Comment history: `acs/cycle1659`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1659/predicates_test.go:3` — above `package cycle1659`

```text
// Package cycle1659 materializes the acceptance criteria of the two inbox items
// this fleet lane committed (lane-scope.json todo_ids; triage-report.md
// ## top_n) — and nothing else (R9.3: the deferred fleet-scope-excluded
// carryovers get ZERO predicates):
//
// Provenance: this contract was RED-authored in cycle 1652 (go/acs/cycle1652,
// never shipped — that cycle's audit stalled on an artifact timeout) and rode
// the ADR-0076 continuation chain 1652 → 1657 → 1659 with its GREEN build in
// the salvage snapshot. Cycle 1659 rebinds the package to its own number so
// `evolve acs suite --cycle 1659` runs it; the predicate bodies are unchanged
// except for the binding-assert wording the phantom-binding classifier pins.
//
//	triage-empty-commitment-still-dispatches-spine  (P1, pipeline-integrity)
//	dossier-producer-params-struct                  (low, techdebt)
//
// Both inbox records were read verbatim from the worktree's .evolve/inbox/
// (the harness's Task Contract block could not open them at the project root,
// so the records' own acceptance arrays are the authority here).
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	T1-AC1 claimable inbox + top_n [] ⇒ no tdd/build/audit dispatch, a named
//	       non-PASS reason                                    → 001
//	T1-AC2 legitimate empty inbox stays SKIPPED via
//	       recordPlannedNoWorkOutcome (regression pin)        → 002
//	T1-AC3 1623-shaped dossier: impossible from the fixed gate
//	       (001's zero-dispatch) + cyclehealth anomaly        → 004, 005
//	T1 anti-no-op: committed work beside a populated inbox
//	       must still advance                                 → 003
//	T2-AC1 writeCycleDossier ≤ 3 params, every field by name   → 006, 008
//	T2-AC2 a new field compiles without editing call sites
//	       (keyed params at both production callers)         → 007, 008
//	T2-AC3 vet + core/dossier suites green; fixed-input bytes
//	       unchanged                                          → 008, 009
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 002 (the
// legitimate no-work disposition must survive — a gate that fails EVERY empty
// commitment passes 001 and breaks this), 003 (a gate keyed on "inbox
// non-empty" bricks every productive cycle), 005 (six healthy dossier shapes
// must raise NO commitment anomaly — legacy records without the tasks field,
// an empty commitment that honestly ended at triage, a missing dossier).
// EDGE/OOD: 001's FAIL-verdict row, 005's resumed-at-triage and retro-only
// rows, 008's omitted-evidence row (no fabricated fields). SEMANTIC: dispatch
// (001), disposition (002), anti-no-op (003), historical detection (004),
// quiet-on-healthy (005), API arity (006), call-site shape (007), byte
// preservation (008), suite health (009) — nine distinct behaviours.
//
// No grep-only predicates (cycle-85 ban): 001/002/003/008 run the composed
// orchestrator (RunCycle / RunCycleFromPhase) through the Builder-frozen
// in-package tests, bound by their `--- PASS:` markers in ONE named package
// with -run narrowing (the cycle-976/1587/1648 shape — internal/core is the
// known-slow suite, so the narrowing is what keeps these cheap); 004/005 run
// the REAL `evolve` binary (built once in TestMain) through its dispatcher —
// the production caller of cyclehealth.Check; 006/007 parse the producer's
// Go AST for the criterion that IS a source-shape property (parameter arity,
// keyed call sites) and are paired with 008, which executes the refactored
// producer against the pre-refactor golden bytes; 009 runs vet and the two
// named packages the inbox record demands.
//
// Reachability probe (cycle-644 rule): this package imports only pkg/acsassert
// and the standard library; acs/cycle1659 is a leaf. The fix's own import
// shapes were compiler-probed at RED time: internal/cyclehealth → internal/
// dossier builds (dossier does not depend on cyclehealth), and internal/core
// already imports internal/inboxbatch (task_contract.go).
```

### `go/acs/cycle1659/predicates_test.go:84` — above `var (`

```text
// ---------------------------------------------------------------------------
// Harness: the real CLI, built once (the cycle-1648 TestMain shape).
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1659/predicates_test.go:135` — above `func assertBoundTestsPass(t *testing.T, pkg string, names ...string) {`

```text
// assertBoundTestsPass runs ONE named package narrowed to exactly the bound
// test names (`-run ^(A|B)$`) from the module root and requires each
// `--- PASS:` marker — the cycle-976/1587/1648 binding shape. A renamed,
// missing, failing, or non-compiling test is classified, never silently green.
```

### `go/acs/cycle1659/predicates_test.go:175` — above `func TestC1659_001_ClaimableInboxWithEmptyCommitmentStopsBeforeImplementationOnBothRoots(t *testing.T) {`

```text
// TestC1659_001 — T1-AC1 on BOTH dispatch roots. The composed cycle: a
// claimable lane item in <ProjectRoot>/.evolve/inbox, triage writes top_n []
// (three rows: silent claim race, cycle-1623's narrated deferral, a FAIL
// verdict over the same evidence), and the orchestrator must dispatch no
// tdd/build/audit/ship, record a NAMED reason distinct from
// triage-empty-commitment, never PASS, never IsTriageNoWorkResult, and
// re-dispatch triage at most once. The resumed root (RunCycleFromPhase) must
// agree — the cycle-1639 fresh/resume divergence lesson.
```

### `go/acs/cycle1659/predicates_test.go:283` — above `func TestC1659_004_HistoricalEmptyCommitmentDossierWithImplementationIsACyclehealthAnomaly(t *testing.T) {`

```text
// TestC1659_004 — T1-AC3 (the historical half) + house rule 2 REACHABILITY.
// A cycle-1623-shaped dossier — tasks [] beside twelve phases — must be
// classified as an anomaly by the PRODUCTION cyclehealth caller (`evolve
// cycle-health`), the anomaly must name an implementation phase, and the
// written cycle-health.json must list the signal in signals_run (so a Scout
// reading the report sees the check ran, not merely that it stayed quiet).
```

### `go/acs/cycle1659/predicates_test.go:542` — above `func TestC1659_008_RefactoredProducerPreservesPreRefactorBytes(t *testing.T) {`

```text
// TestC1659_008 — T2-AC1 + T2-AC3 executed. The refactored producer, fed the
// golden's fixed input through cycleDossierParams, writes the JSON and
// Markdown the eleven-argument producer wrote (golden captured at HEAD
// 287aa81c through the OLD signature); a caller naming only the fields it has
// compiles, runs, and fabricates nothing. The golden files must not be
// gitignored (cycle-93: a testdata file dropped at ship is a silently green
// contract).
```
