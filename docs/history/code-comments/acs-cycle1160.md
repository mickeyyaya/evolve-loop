# Comment history: `acs/cycle1160`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1160/predicates_test.go:3` — above `package cycle1160`

```text
// Package cycle1160 materialises the acceptance criteria for the three tasks
// triage COMMITTED to this fleet lane (triage-report.md `## top_n`):
//
//   - retire-dead-lifecycle-surface       → 001, 002, 003
//   - document-adr0079-shared-root-risk   → 004, 005
//   - materialize-cycle1158-acs-predicates → 006, 007
//
// `## deferred` and `## dropped` are both empty this cycle, so every predicate
// here binds to committed work (R9.3 floor-binding).
//
// # What is left of the cycle-1156 audit
//
// D1 (BLOCKING: the PASS promote loop early-returned past the residual drain)
// and D2 (`bumpFailureCount` ungated on `systemLevel`, ADR-0072 AC4) landed on
// main in cycle 1157 (`ea1d0006`) and are verified live in this worktree. What
// the audit left open is the cheap half:
//
//   - D3 — `CycleOutcome.LaneIDs` (outcome.go:38) has no reader anywhere in
//     production, and `ReleaseCycleProcessingWithQuarantine` (inboxmover.go:638)
//     has no production caller: `ApplyCycleOutcome`'s FAIL path owns the drain
//     now and calls the unexported `releaseCycleProcessing` core directly. Two
//     public entry points into one lifecycle is exactly the drift the seam was
//     built to prevent (never_duplicate_centralize).
//   - D4 — `ClaimLaneScope` writes into the SHARED inbox root from a per-lane
//     closeout path, while sibling lanes' triage reads that same root with no
//     lane isolation (triage.go:113). At width 3 that is a real, bounded
//     cross-lane miss window, and ADR-0079 does not mention it at all.
//
// # Predicate quality (cycle-85 ban)
//
// No predicate here is satisfiable by adding a magic string to a source file.
// 001 reflects over the real `inboxmover.CycleOutcome` type; 002 asks the Go
// toolchain for the package's actual documented API surface and compiles the
// dependent ACS packages; 003 and 005 drive `ApplyCycleOutcome` /
// `ClaimLaneScope` against real temp trees and assert on where items physically
// land and what their durable failure_count says; 006 and 007 run the cycle-1158
// suite as a subprocess and assert on its exit code and per-test verdicts.
//
// 004 is the single content assertion, and it is legitimate rather than
// degenerate: the ADR text IS the deliverable for that task (there is no
// behavior to call), and 005 pins the behaviour the prose claims so the two
// cannot drift — a doc that names the risk while the code stopped exhibiting it
// fails 005, and code that exhibits it while the ADR stays silent fails 004.
```

### `go/acs/cycle1160/predicates_test.go:108` — above `func testOpts(root string, stderr io.Writer) inboxmover.Options {`

```text
// testOpts returns inboxmover Options rooted at root with the landing gate
// stubbed to "landed". The real gate shells out to `git merge-base` and is
// fail-open on a non-git dir; stubbing it keeps these predicates asserting the
// LIFECYCLE rather than incidental git behaviour of a temp dir.
```

### `go/acs/cycle1160/predicates_test.go:252` — above `func TestC1160_003_apply_cycle_outcome_drain_survives_the_retirement(t *testing.T) {`

```text
// AC (anti-overcorrection twin): deleting the wrapper must not delete the drain
// BEHAVIOUR it fronted. The obvious overcorrection — removing the quarantine
// path along with its dead entry point — re-opens `wave-lane-task-quarantine-dead`
// exactly, so this predicate drives `ApplyCycleOutcome` end to end and asserts
// on the filesystem: committed ids bump, uncommitted menu ids do not, the
// ceiling still parks a poison item, and a system-level FAIL still bumps nothing
// (ADR-0072 AC4, the cycle-1157 repair).
//
// Expected pre-existing GREEN at RED time: it is the regression fence around
// Task 1, not a criterion Task 1 introduces.
```

### `go/acs/cycle1160/predicates_test.go:312` — above `root3, inbox3 := newInbox(t)`

```text
// SystemLevel: no bump at all (ADR-0072 AC4, the cycle-1157 D2 repair).
```

### `go/acs/cycle1160/predicates_test.go:339` — above `func TestC1160_004_adr0079_documents_shared_root_mutation_risk(t *testing.T) {`

```text
// AC (RED): "ADR-0079 records the ClaimLaneScope shared-inbox-root mutation as
// an accepted risk, naming the sibling-lane miss window and BOTH bounding
// mechanisms."
//
// The ADR argues (correctly) that claiming at outcome time rather than at
// dispatch avoids starving triage. What it never says is the cost of that
// placement: the claim moves files OUT of the shared inbox root while sibling
// lanes are live, and triage reads that root with no lane isolation
// (triage.go:113). At standing width 3 a sibling's triage can miss an item for
// one cycle. The audit offered "acknowledge (accepted risk) or guard"; guarding
// adds locking to a self-healing, bounded window, so the decision is to
// document.
//
// The `## Verification` requirement is what stops this from being a one-liner:
// a risk paragraph that names no mechanism is not an accepted risk, it is a
// shrug. 005 pins the behaviour these words describe.
```

### `go/acs/cycle1160/predicates_test.go:472` — above `func TestC1160_006_cycle1158_predicates_exist_and_pass(t *testing.T) {`

```text
// AC (RED): "`go test -tags acs -count=1 ./acs/cycle1158/` — all 7 predicates
// TestC1158_001..007 PASS."
//
// Cycle 1158 authored the eval
// (.evolve/evals/land-cycle-1156-lifecycle-seam-with-audit-fixes.md) but ended
// WARN on an unrelated `debugger` phase failure before materialising it, so the
// whole cycle-1156 repair — D1's collected-not-early-returned promote errors,
// D2's systemLevel gate, and now D3/D4 — has an eval score-cap pointing at a Go
// package that does not exist. Every one of those `max_if_missing` caps is
// therefore live against a missing file.
//
// Asserting on the per-test verdicts rather than just the exit code is what
// rejects the cheap green: an empty package with a single trivial test also
// exits 0.
```

### `go/acs/cycle1160/predicates_test.go:505` — above `func TestC1160_007_cycle1158_predicates_excluded_without_the_acs_tag(t *testing.T) {`

```text
// AC (negative / tag correctness): the cycle-1158 package must be EXCLUDED from
// the normal suite.
//
// ACS predicates are state assertions, not unit tests — "the ADR documents X"
// is false mid-edit — so a package that leaks into `go test ./...` red-fails CI
// on correct work. `acssuite.TestAllACSPredicatesAreTagged` enforces the tag's
// presence in the normal suite; this predicate enforces its EFFECT, which is
// the property that actually matters and the one a stray second file in the
// package could break without touching predicates_test.go.
```
