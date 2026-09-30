# Comment history: `acs/cycle1620`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1620/predicates_test.go:3` — above `package cycle1620`

```text
// Package cycle1620 materializes the cycle-1620 acceptance criteria for the one
// task this fleet lane committed (lane-scope.json todo_ids; triage-report.md
// ## top_n): `multi-slug-lane-scope-reconciliation`. Per R9.3 nothing here binds
// to the deferred `per-slug-lane-splitting` or `out-of-lane-carryovers` items —
// they get ZERO predicates.
//
// The defect (cycle-1480 batch-20260815c wave-2, recurred cycle-1483). A lane
// bundling `minted-phase-verdict-contract-unsatisfiable` + `dead-api-sweep` had
// TDD mint a cycle-wide predicate suite covering BOTH members while the
// Builder's deliverable contract bound only the FIRST. Nothing reconciled the
// two scopes, so the lane ran the full ~12-phase spine and FAILed at audit with
// slug 2 entirely undelivered. Reproduced this cycle in
// .evolve/runs/cycle-1620/bug-reproduction-report.md (the enforce-stage reviewer
// returns Approve:true on the two-member/one-declared shape).
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 2-slug lane delivers both or blocks at TDD->Build  → C1620_001..004
//	AC2 declared set == committed set, deterministic check  → C1620_005, C1620_006
//	    with a red-first test pinning the cycle-1480 shape
//	AC3 single projection source, no second lane-scope parser → C1620_007
//	AC4 go vet + touched suites + -race green               → C1620_008
//	(durability, cycle-1501 lesson: the recurring slug's eval) → C1620_009
//
// Adversarial axes. NEGATIVE: C1620_002 and C1620_003 — the cheapest way to pass
// every block predicate is to reject all multi-member lanes (or every lane), which
// kills the fleet's only legitimate bundling path and every single-slug cycle;
// both must still proceed. EDGE/OOD: C1620_006 — a COMPLETE handoff shown inside
// an outer `~~~markdown` example fence is illustration, not declaration, and must
// not be mistaken for the real one (the 2026-09-09 recovery review's false-accept
// control). SEMANTIC: blocking, proceeding, production arming, projection
// single-sourcing, toolchain health and durable memory are six distinct
// behaviors, not one behavior restated.
//
// No grep-only predicates (the cycle-85 ban): C1620_001..003 and C1620_006 drive
// the REAL production reviewer constructor at the REAL enforce stage and assert
// on its verdict; C1620_004 and C1620_007 call real functions and assert on
// returned values (their file assertions are auxiliary); C1620_005 and C1620_008
// execute the toolchain and require named markers / exit codes; C1620_009 runs
// the eval's own graders.
```

### `go/acs/cycle1620/predicates_test.go:67` — above `redFirstTest = "TestTDDScopeGate_TwoSlugHandoffOmittingCommittedMemberBlocks"`

```text
// redFirstTest is the AC2-mandated red-first unit test pinning the
// cycle-1480 shape, and redFirstFile is where it lives.
```

### `go/acs/cycle1620/predicates_test.go:153` — above `func TestC1620_001_two_slug_lane_blocks_at_tdd_build_boundary(t *testing.T) {`

```text
// AC1: the cycle-1480 shape. Two committed members, one declared — Build must
// not start, and the block must NAME the undelivered member so the operator
// does not have to re-derive the omission from a diff.
```

### `go/acs/cycle1620/predicates_test.go:201` — above `func TestC1620_004_gate_is_armed_in_the_production_composition(t *testing.T) {`

```text
// AC1 (WIRING PROOF): the reconciliation must be reachable from the PRODUCTION
// composition, not just from a test. cmd/evolve/cmd_cycle.go appends
// topngate.NewReviewer(cfg.TopNGate) to the cycle's reviewer chain, and the
// resolved gate stage must be enforce by default — a correct gate wired at
// StageOff (or unregistered) blocks nothing and the cycle-1480 burn recurs.
```

### `go/acs/cycle1620/predicates_test.go:217` — above `func TestC1620_005_red_first_test_pins_the_cycle_1480_shape(t *testing.T) {`

```text
// AC2: the red-first test pinning the cycle-1480 shape must exist IN THE REPO,
// be git-tracked (an untracked reproducer is dropped at ship — the cycle-92
// class), and PASS. One named test in ONE small package, -run-narrowed; the
// named `--- PASS:` marker is required because `go test -run` exits 0 when it
// matches nothing.
```

### `go/acs/cycle1620/predicates_test.go:240` — above `func TestC1620_006_outer_fence_fake_complete_handoff_rejected(t *testing.T) {`

```text
// AC2 (ADVERSARIAL / OOD): a report that DOCUMENTS a complete handoff inside an
// outer `~~~markdown` example fence while declaring one member must still block.
// Illustration is not declaration; the live gate's fence walker takes the FIRST
// DECLARING JSON fence inside the real handoff section (a fence carrying neither
// slugs nor testFiles never counts — cycle-1620 audit M1), which is exactly how
// a fake complete declaration inside an example fence must not evade the check
// (2026-09-09 recovery review).
```

### `go/acs/cycle1620/predicates_test.go:271` — above `func TestC1620_007_one_lane_scope_projection_for_both_consumers(t *testing.T) {`

```text
// AC3: ONE projection source yields the lane's slug set. The behavioral half
// calls the surviving exported consumer entry point and pins its contract
// (member ORDER preserved; fail-open to nil on an absent/malformed pin — a
// projection that aborts a lane on a broken pin recreates the cycle-760..762
// destruction class). The structural half is the AC's own literal requirement
// ("grep-proof: no second parser of lane-scope slugs"): exactly ONE non-test Go
// file may declare the lane-scope wire shape: go/internal/core/lanescope.go
// (cycleoutcome and the audit defect ledger read it through core.LaneScopeIDs).
```

### `go/acs/cycle1620/predicates_test.go:354` — above `func TestC1620_009_eval_pins_the_class_durably(t *testing.T) {`

```text
// The recurring slug's durable memory (cycle-1501 lesson: an eval re-authored
// per attempt inside a discarded worktree lets attempt N+1 drop the criterion
// attempt N was graded CRITICAL on). The eval must be tracked, must clear the
// SSOT rigor checker, must carry one grader per behavioral criterion, and its
// FIRST grader is EXECUTED here so a decorative evidence string cannot stand in
// for a runnable one.
```
