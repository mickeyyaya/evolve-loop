# Comment history: `acs/cycle459`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle459/predicates_test.go:3` — above `package cycle459`

```text
// Package cycle459 materialises the cycle-459 acceptance criteria for the
// single triage-committed task `triagecap-prose-counter-defect` (inbox
// 2026-07-02, post-mortem of cycles 448/449): the triage-cap gate defect
// chain F1–F5 in go/internal/triagecap (floors.go, reviewer.go, demotion.go).
//
// AC map (1:1, R9.3 floor-binding; predicates for the one ## top_n task only —
// the fleet-policy tasks were deferred by triage and get ZERO predicates):
//
//	AC1  F1 golden: the EXACT cycle-449 report counts 3, not 7   → C459_001 (golden, direct call)
//	AC2  F1 semantic: evidence citations never count as floors    → C459_002 (negative, direct call)
//	AC3  F1 anti-weakening: true multi-floor commitments still
//	     count fully (cycle-448 golden = 4; cycle-283 stays 12)   → C459_003 (edge, direct call)
//	AC4  F2: reject reason states the declaration escape
//	     (triage-decision.json committed_floors[])                → C459_004 (semantic, named unit test)
//	AC5  F5: reject reason lists the counted packages             → C459_005 (semantic, named unit test)
//	AC6  F4: reset-sealed cycle is a transparent gap — the
//	     448/449 pair demotes cycle 451                           → C459_006 (positive, named unit test)
//	AC7  F4: relief stays one cycle; stale pairs outside the
//	     window keep enforcing                                    → C459_007 (negative, named unit tests)
//	AC8  F3: floor-bearing report without committed_floors
//	     declaration WARNs (and only then)                        → C459_008 (negative+edge, named unit test)
//	AC9  F3: the triage prompt instructs emitting the companion   → C459_009 (config-check, pre-existing GREEN pin)
//	AC10 regression: triagecap vet + -race suite green            → C459_010 (regression)
//
// 1:1 enforcement: 10 predicates + 0 manual + 0 removed = 10 ACs, each AC
// exactly one disposition, none double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"): C459_001/002
// fail directly on the unfixed counter (7 and 4 instead of 3 and 1).
// C459_004..008 shell to this cycle's named RED unit tests in
// internal/triagecap (gate_defect_chain_test.go) guarded by requireTestsRan,
// so an unwritten or renamed test can never green them. C459_010 fails while
// any triagecap unit test is red. C459_003 and C459_009 are pre-existing
// GREEN pins: C459_003 is the anti-weakening bound that keeps the gate's
// purpose intact (real overpacking must still count), and C459_009 pins the
// prompt instruction the F3 producer check depends on.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C459_002 (evidence prose must NOT count), C459_007 (stale
//	            pair must NOT demote; relief must NOT extend past one cycle),
//	            C459_008's silent subtests (warning must NOT fire with a
//	            declaration or without floors)
//	Edge/OOD:   C459_003 (multi-target item = boundary of "scoped counting"),
//	            C459_006 (missing middle cycle = the reset hole)
//	Semantic:   C459_004/005 (actionable corrective is distinct behavior from
//	            rejecting), C459_001 vs C459_003 (3-not-7 AND 4-stays-4 are
//	            only jointly satisfiable by target-scoped counting)
```

### `go/acs/cycle459/predicates_test.go:65` — above `var goldenVocab = []string{`

```text
// goldenVocab mirrors the production package vocabulary relevant to the
// cycle-448/449 goldens: the true floor targets plus the phantom sources the
// defective counter attributed (scout, sysexec) and distractors. Must match
// gapGoldenPkgs in internal/triagecap/gate_defect_chain_test.go.
```

### `go/acs/cycle459/predicates_test.go:113` — above `func TestC459_001_GoldenCycle449CountsThreeFloors(t *testing.T) {`

```text
// TestC459_001_GoldenCycle449CountsThreeFloors (AC1, golden, behavioral —
// invokes the counter directly): the EXACT preserved cycle-449 triage report
// committed three coverage floors (core 85.0 / bridge 94.5 / audit 96.0, one
// per top_n item); the gate counted 7 because evidence citations named other
// packages with percentages. RED today: CountCommittedFloors returns 7.
```

### `go/acs/cycle459/predicates_test.go:138` — above `func TestC459_003_TrueMultiFloorCommitmentStillCountsFour(t *testing.T) {`

```text
// TestC459_003_TrueMultiFloorCommitmentStillCountsFour (AC3, edge,
// anti-weakening pin — pre-existing GREEN): cycle 448 genuinely committed
// four floor targets ("coverage floors core ≥85.0%, audit ≥96.0%, bridge
// ≥94.5%" + "core total coverage ... ≥86.0%"). The F1 fix must not collapse
// true multi-package commitments — the gate still guards real overpacking
// (cycles 280/282/283). Must count 4 before AND after the fix.
```

### `go/acs/cycle459/predicates_test.go:175` — above `func TestC459_006_ResetSealedGapStillDemotes(t *testing.T) {`

```text
// TestC459_006_ResetSealedGapStillDemotes (AC6/F4, positive): the incident
// replay — same-template rejections recorded at 448 and 449, cycle 450
// reset-sealed without a record, review at 451 must demote to shadow and
// auto-file the defect. RED today: ShouldDemote demands records at
// currentCycle-1 AND -2.
```

### `go/acs/cycle459/predicates_test.go:226` — above `func TestC459_010_TriagecapRegressionVetAndRace(t *testing.T) {`

```text
// TestC459_010_TriagecapRegressionVetAndRace (AC10, regression): the touched
// package must be vet-clean and fully -race green — including every
// pre-existing replay pin (cycle-283 stays 12, cycle-301 stays 2, adjacent-
// pair demotion still fires) and this cycle's contract tests. RED today
// while the F1–F5 unit tests are red; GREEN only when the whole package is.
```
