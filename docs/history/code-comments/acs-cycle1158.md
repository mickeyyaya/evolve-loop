# Comment history: `acs/cycle1158`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1158/doc.go:1` — above `package cycle1158`

```text
// Package cycle1158 holds the ACS predicates for the cycle-1158 eval
// (.evolve/evals/land-cycle-1156-lifecycle-seam-with-audit-fixes.md). The
// predicates themselves live in predicates_test.go behind `//go:build acs`.
//
// This file carries NO build tag on purpose. ACS predicates are environment
// assertions ("the ADR documents X", "the export is retired"), not unit tests,
// so they must not run in the ordinary suite — but a directory whose only file
// is tag-excluded fails to build at all ("build constraints exclude all Go
// files"), which turns a correctly-tagged package into a red `go test ./...`.
// One untagged, empty package clause makes the package build away to nothing
// without `-tags acs` instead of failing, which is the property
// go/acs/cycle1160's predicate 007 asserts.
```

### `go/acs/cycle1158/predicates_test.go:3` — above `package cycle1158`

```text
// Package cycle1158 materialises
// .evolve/evals/land-cycle-1156-lifecycle-seam-with-audit-fixes.md — the eval
// cycle 1158 authored for the four defects the cycle-1156 audit raised against
// `inboxmover.ApplyCycleOutcome`, the single cycle-outcome lifecycle seam.
//
// Cycle 1158 ended WARN on an unrelated `debugger` phase failure before it could
// write this file, so all seven `score_cap` entries have been pointing at a Go
// package that did not exist: every cap was live against a missing target and
// therefore unenforceable. This package closes that gap. Predicate numbering is
// fixed by the eval's evidence commands (TestC1158_001..007) — a rename leaves
// the corresponding cap unenforceable again.
//
//	001 — D1 (BLOCKING): a PASS-path promote error still drains residual claims
//	002 — D1: the promote loop attempts EVERY committed id after a failure
//	003 — D1 aggravator: the ship phase never claims a drain it did not complete
//	004 — D2: a system-level FAIL never bumps the durable failure_count (AC4)
//	005 — D2 twin: a task-level FAIL still bumps it (the S5 ceiling stays live)
//	006 — D3: the production-dead lifecycle surface is retired
//	007 — D4: ADR-0079 records the ClaimLaneScope shared-root mutation risk
//
// # Predicate quality (cycle-85 ban)
//
// None of these is satisfiable by adding a magic string to a source file. 001,
// 002, 004 and 005 drive the real `ApplyCycleOutcome` over temp trees and assert
// on where items physically land and what their durable `failure_count` says;
// 003 runs the ship package's own regression tests as a subprocess and asserts
// on their per-test verdicts; 006 reflects over the real `CycleOutcome` type and
// asks the Go toolchain for the package's actual exported API. 007 is the single
// content assertion and is legitimate: the ADR prose IS the deliverable for D4,
// and `go/acs/cycle1160`'s predicate 005 pins the behaviour that prose describes
// so the two cannot drift.
```

### `go/acs/cycle1158/predicates_test.go:99` — above `func blockDir(t *testing.T, path string) {`

```text
// blockDir writes a regular FILE where a directory is needed, so the
// destination MkdirAll inside Promote fails — the infrastructure non-delivery
// ADR-0079 made loud.
```

### `go/acs/cycle1158/predicates_test.go:176` — above `func TestC1158_001_pass_promote_error_still_drains_residual_claims(t *testing.T) {`

```text
// Criterion (cap 9/10, BLOCKING): "A PASS-path promote error still drains
// residual claims back to the inbox root (no early return)."
//
// The cycle-1156 defect was a bare `return` inside the PASS promote loop. One
// unwritable processed/cycle-N/ stranded not just the failing id but every item
// already parked in processing/cycle-N/, reintroducing the cross-cycle orphan
// shape of cycles 124/265/294/295/308 that promoteInbox's own invariant comment
// forbids. The contract is BOTH halves at once: the error still reaches the
// caller, AND the drain has already run by the time it does.
```

### `go/acs/cycle1158/predicates_test.go:275` — above `func TestC1158_004_system_level_failure_never_bumps_failure_count(t *testing.T) {`

```text
// --- D2: ADR-0072 AC4, and its anti-overcorrection twin ---------------------
```

### `go/acs/cycle1158/predicates_test.go:277` — above `func TestC1158_004_system_level_failure_never_bumps_failure_count(t *testing.T) {`

```text
// Criterion (cap 7/10): "A system-level FAIL never increments the durable
// failure_count (ADR-0072 AC4)."
//
// This seam is what first makes bumpFailureCount reachable for wave lanes, so an
// ungated bump lets the documented recurring quota-storm class (cycles
// 1077-1096) walk healthy committed ids toward TaskRetryCeiling — after which a
// single later task-level FAIL quarantines a backlog that never failed on its
// own merits. systemLevel gates the BUMP, not merely the quarantine decision.
//
// The fixture is the exact boundary: failure_count 1 against ceiling 2, so an
// ungated bump would both increment AND quarantine. Asserting on the durable
// count (not just "did it quarantine") is what separates a real AC4 gate from a
// gate applied only at the quarantine branch.
```

### `go/acs/cycle1158/predicates_test.go:404` — above `func TestC1158_007_adr0079_documents_shared_root_mutation_risk(t *testing.T) {`

```text
// Criterion (cap 4/10): "ADR-0079 records the ClaimLaneScope shared-inbox-root
// mutation as an accepted risk."
//
// ADR-0079 argues (correctly) that claiming at outcome time rather than at
// dispatch avoids starving triage. What it never said is the cost: the claim
// moves files OUT of the shared inbox root while sibling lanes are live, and
// triage reads that root with no lane isolation (triage.go:113). At the standing
// fleet width of 3 a sibling's triage can miss an item for one cycle.
//
// The bounding-mechanism requirement is what stops this from being a one-liner:
// a risk paragraph naming no mechanism is not an accepted risk, it is a shrug.
```
