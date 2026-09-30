# Comment history: `acs/cycle1663`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1663/predicates_test.go:3` — above `package cycle1663`

```text
// Package cycle1663 materializes the acceptance criteria of the ONE inbox item
// this fleet lane committed (lane-scope.json todo_ids ∩ triage-report.md
// ## top_n) — and nothing else (R9.3):
//
//	lost-ship-dossier-evidence  (high, weight 0.82, feature)
//
// The lane's second scoped id, dossier-producer-params-struct, was triage-
// DROPPED as already shipped (cycleDossierParams is live at
// go/internal/core/dossier_producer.go, inbox record consumed via ship
// 2026-09-13) and gets ZERO predicates here; its own contract still runs as
// the landed core tests 003 names.
//
// The defect. finalizeCycle stamps the landing-lost SystemFailureSignal onto
// CycleResult and downgrades the verdict to WARN (lost_landing_floor.go,
// PR #482) — but writeCycleDossier receives only the outcome string, so the
// committed knowledge-base/cycles/cycle-N.{json,md} carries a WARN
// indistinguishable from any other WARN and the operator cannot see WHY.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 landing-lost dossier carries the signal's category AND evidence,
//	    through the REAL writeCycleDossier path (both production callers)  → 001
//	AC2 the landed sibling with the same transient ship error carries none;
//	    an ordinary PASS dossier stays byte-clean                         → 002
//	AC3 acs/cycle1544 predicates 004-005 restored and green               → 004
//	house-rule floor: landed dossier/floor tests not weakened; the schema
//	    drift guard (Go struct ⇄ schemas/cycle-dossier.schema.json) stays
//	    green once the new field exists                                   → 003
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 002 — the
// cycle-1536 sibling recorded the SAME GIT_FLEET_REBASE_NEEDED and LANDED;
// a sink that keys on "ship-error present" instead of the floor's signal
// passes 001 and fails this. EDGE/OOD: 002's ordinary-PASS golden (nonexistent
// workspace ⇒ never shipped ⇒ nothing to lose; the bytes must equal the
// pre-change producer's — no `null`, no empty section), 001's abnormal-exit
// row (a signal stamped before an abort must survive the epilogue's own
// dossier) and its no-signal twin (nothing fabricated on the FAIL path).
// SEMANTIC: evidence durability (001), false-alarm immunity (002), schema +
// anti-weakening floor (003), historical-suite restoration (004) — four
// distinct behaviours.
//
// No grep-only predicates (cycle-85 ban): every seam here (completeCycle,
// abnormalEpilogue, writeCycleDossier, the dossier build) is unexported
// inside internal/core / internal/dossier, so each predicate drives it via
// the sanctioned behavioural-via-subprocess shape — a `-run`-narrowed,
// `-count=1`, single-named-package `go test -v` that must print
// `--- PASS: <name>` for every Builder-frozen binding test. Asserting on the
// PASS LINE, never exit 0, is load-bearing: a pattern matching NO test exits
// 0 with "no tests to run", so a still-missing binding would false-GREEN.
//
// Flaky-shape contract: ONE named package per invocation, never `/...`, never
// a whole-package run of the 40s+ core suite; no wall-clock bounds, no
// literal PIDs, no bare git.
//
// Reachability probe (cycle-644 rule): this package imports only
// pkg/acsassert and the standard library — a leaf. The frozen core test
// references `dossier.ParseJSON`/`dossier.VerdictWarn` from package core,
// which ALREADY imports internal/dossier (dossier_producer.go) — no new edge;
// and internal/dossier already imports internal/cyclestate (the signal type's
// home), so a dossier field typed on the signal is buildable without touching
// core→dossier→core. Compiler-probed at RED time: `go vet ./internal/core/`
// green with the frozen test in place.
```

### `go/acs/cycle1663/predicates_test.go:114` — above `func TestC1663_001_LostLandingEvidenceReachesTheCommittedDossier(t *testing.T) {`

```text
// TestC1663_001_LostLandingEvidenceReachesTheCommittedDossier — AC1. The
// binding tests drive cycle-1535's REAL ship artifacts (ship-error
// GIT_FLEET_REBASE_NEEDED, no ship-binding) through cycleRun.completeCycle —
// finalizeCycle fires the floor, writeCycleDossier commits the record — and
// assert the committed JSON carries a `system_failure` object whose
// `category` is "landing-lost" and whose `evidence` equals the floor's
// evidence VERBATIM, with the Markdown half naming both; and that the
// second production caller, cycleRun.abnormalEpilogue, threads a signal
// already stamped on the result the same way (and fabricates nothing when
// there is none). The verdict routing (WARN) is asserted unchanged: this is
// evidence added, not a re-classification.
```

### `go/acs/cycle1663/predicates_test.go:132` — above `func TestC1663_002_LandedSiblingAndOrdinaryPassCarryNoEvidence(t *testing.T) {`

```text
// TestC1663_002_LandedSiblingAndOrdinaryPassCarryNoEvidence — AC2. The same
// real closeout over cycle-1536's artifacts (same transient error + a
// binding for commit adcbddb2) must commit a PASS dossier with NO
// system-failure object and no leaked "landing-lost"/error-code text — and
// the binding's commit_sha, proving the producer read THAT workspace. And
// the ordinary-PASS golden (testdata/dossierparams/cycle-4243.golden.*,
// captured from the producer at b9c0df76 BEFORE any signal field existed)
// must still match byte for byte: a nil signal adds nothing to the record.
```

### `go/acs/cycle1663/predicates_test.go:179` — above `func TestC1663_004_Cycle1544PredicatesRestoredAndGreen(t *testing.T) {`

```text
// TestC1663_004_Cycle1544PredicatesRestoredAndGreen — AC3, verbatim from the
// inbox record: "acs/cycle1544 predicates 004-005 restored and green". The
// 2026-08-23 console salvage excised 001-005 from that suite; 004-005 (the
// dossier-evidence half) are restored this cycle and must print their own
// PASS lines under the acs tag. RED today for the right reason: before this
// cycle the pattern matched no test ("no tests to run", exit 0) — the exact
// false-GREEN shape asserting on the PASS line exists to catch.
```
