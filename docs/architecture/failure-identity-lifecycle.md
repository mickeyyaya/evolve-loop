# Failure Identity & Carryover Lifecycle

> Cycle-1440, and §4 on 2026-10-27. Covers the four seams that decide **what counts as the same failure**
> and **when a remembered failure is done**. The seams are carryover retirement, staging-refusal
> classification, reason fingerprint normalization and the failure class fingerprint.

Four subsystems answer one question — *is this the same defect I already saw?* —
and each had a leak that made the loop burn cycles re-working closed items.

## 1. Carryover retirement (PASS closeout)

`state.json:carryoverTodos[]` is the cross-cycle memory of open follow-ups, and the
router prompt renders its first 20 entries. Before this change `mergeCarryoverTodos`
(`go/internal/core/failure_learning.go`) only ever **unioned** disk with incoming. It
deduped by ID. Nothing ever removed an entry.

An entry whose work actually shipped
persisted forever. The 2026-08-10 investigation found that 124 of 254 live entries were
stale duplicates of a handful of classes. The router window was therefore mostly done work.

**Seam:** `core.RetireCarryoverTodos(todos, committedIDs) []CarryoverTodo`.

An entry retires when either:

- its `ID` is in `committedIDs` (the ids this cycle actually committed to), or
- it shares a retired entry's **cross-cycle Action fingerprint**
  (`carryover.fingerprint` (internal/core/carryover/identity.go, ADR-0103 unit 03) — Action text with cycle tokens folded and
  whitespace/case normalized). These are the per-cycle re-mints of the same class
  that the ID-keyed dedupe never collapsed.

Everything else survives in original order (the router window is ordered). The
function never mutates the input slice. Callers re-read state under a lock. A mutated input is dangerous:
it corrupts the merge of a concurrent peer. A blank committed id is malformed input. It is not
a claim about the equally-malformed blank-ID entry, and it retires nothing.

**Production caller:** `retireCommittedCarryover` in
`go/internal/phases/ship/postship.go`, invoked from `promoteInbox` inside the landed
branch. Two properties are load-bearing:

- It rides the **same landing gate** as inbox promotion (cycle-598). Without that gate, retiring on an
  unlanded ship erases the only record of work that never shipped.
- It is **bookkeeping only**: a missing `state.json`, an unreadable state map, or a
  failed write is a `WARN` in `res.Logs`, never a ship error.

It rewrites the *raw* JSON entries and does not round-trip them through the typed
struct. A field that this binary does not model therefore survives retirement. It takes the
same `withStateLock` that every other `state.json` RMW takes (ADR-0049 S2 / G2).

## 2. Two-strikes staging refusal

`stageExplicitPaths` (`go/internal/phases/ship/gitops.go`) classified **every**
refused `git add` as `core.ShipClassTransient`, so the failure floor kept
re-dispatching refusals that were never able to succeed in place. Cycle 1365 burned its
entire retry budget on one `.evolve/evals` pathspec: its worktree base predated the
`.gitignore` carve-out, so no retry was able to win.

**Rule:** the *first* refusal of a given pathspec stays `ShipClassTransient` — a
genuinely flaky add must keep its retry. A *second consecutive* refusal of the
**same pathspec** reclassifies to `ShipClassPrecondition`: deterministic, so the
router routes to continuation/salvage instead of another doomed try.

- Identity is the **sorted path set** (`stageRefusalKey`), so "same pathspec" is
  order-independent but a different refused path is a different failure and resets
  to transient. If a rule merely counted refusals, it wrongly kills the retry for
  an unrelated second failure.
- Memory is **workspace-scoped** (`<workspace>/ship-stage-refusal.txt`): fleet lanes
  run concurrently, so one lane's strike must never block a peer's first try.
- With **no workspace** there is nowhere to record a strike, so classification
  degrades to the pre-existing transient behavior rather than guessing deterministic.
- A successful staging call clears the memo, so *consecutive* means consecutive.

## 3. Reason fingerprint normalization

`normalizeReasonForFingerprint` (`go/internal/core/failure_digest.go`) projects a
reason onto its **defect identity** for the identical-fingerprint breaker
(`IdenticalFingerprintCeiling = 3`). Display and identity are two projections of one
reason string. The digest, the dossier and `audit-fail-reason.json` all keep the
reason verbatim. Only the hash input is normalized.

It previously stripped two tokens (`narrative=<verdict>`, go-test durations). Every
other per-cycle-varying token still split ONE recurring defect into N fingerprints,
so the breaker never reached its ceiling and the batch kept burning cycles. Two
further tokens now fold:

| Token | Shape | Why |
|---|---|---|
| `cycleNumberToken` | `.evolve/runs/cycle-1365/…`, `.evolve/worktrees/cycle-42824668-1440/…`, bare `cycle 1365` | The same abort recorded on two cycles is one defect. The trailing `(?:-\d+)*` is required for the worktree shape, whose name carries both a lane hash and a cycle number. |
| `attemptDenominatorToken` | `(attempt 1/3)`, `retry 3 of 4` | One unwinnable defect retried N times is one defect — exactly the retry storms the breaker exists to halt. |

**Blast radius is the constraint, not the folding.** If over-normalization
collapses two different defects, it blinds the breaker far worse than the variance
it fixes. Thus:

- Only the *number* folds — the path around it (which directory, which artifact
  **file**) is untouched, so two artifacts in one cycle dir stay two defects.
- The `attempt` keyword is preserved with `${1}`, so an `attempt` reason and a `retry`
  reason — different writers, different failures — cannot collapse into each other.
- The pre-existing `narrative=<verdict>` and duration pins stay green.

The negative tests (`DistinctDefectsStayDistinct`,
`TouchesOnlyTheNarrativeToken`, `DifferentPathspecStaysTransient`) refute a
degenerate implementation. A normalizer that returns a constant, or a strike rule that
merely counts, satisfies every positive test above.

## 4. Failure class fingerprint

A failure digest has two fingerprints. The exact fingerprint identifies one reason text. The class fingerprint identifies one root cause across cycles.

**Problem (2026-10-27).** Cycles 2043, 2046 and 2055 failed on one pipeline-code defect. The explanation floor returns before the composed floor runs, so the single correction sees only part of the failures. Each reason names other files, functions and percentages. Thus each cycle got a new exact fingerprint (`0a48aa5747ca`, `c98f5503ae68`, `6e9e4730e355`) and recurrence 0. `RecurrenceConsoleFloor` and the identical-fingerprint breaker did not trip.

| Fingerprint | Shape | Input | Use |
|---|---|---|---|
| Exact | `<phase>\|<pre-class>\|<hash>` | phase, pre-class and the normalized reason text | the disposition cross-check, the ack ledger, the exact recurrence |
| Class | `<phase>\|<pre-class>\|class-<hash>` | phase, pre-class, the sorted names of the failing floor checks and the disposition layer | the class recurrence, the console floor and the blocker breaker |

**Failing checks.** `failingChecks` (`go/internal/core/failure_class.go`) reads each build handoff floor reason. A failure line that starts with `explanationdocs.FailurePrefix` (`Explanation Documentation: `) names `explanation-documentation`. The prefix inside a failure does not count.

Each producer in `go/internal/explanationdocs` builds its failure from that constant. `TestFailurePrefix_EveryChangedAreaFailureStartsWithIt` pins the Changed Areas producer. A floor failure count above the count of those lines names `composed-floor`. A reason from another gate names no check.

The floor and the parser share the header constants `floorReasonHead` and `floorReasonCount` (`go/internal/core/build_handoff_floor.go`). `TestFailingChecks_ReadsTheReasonThatTheFloorWrites` sends a real `BuildHandoffFloor.Review` reason through the parser.

`AssembleFailureDigest` writes the names to the `checks` field of the digest.

**Order.** The digest is written before the retro, but the layer comes from the retro `disposition.json`. Thus the class is recorded after the retro, in `finalizeRetroCompletion`, when `VerifyDisposition` passes. `recordFailureClass` reads the layer from the disposition. It does not start a second LLM call. It writes the class fields to the harness digest record and to the workspace digest.

**Later assembly.** A cycle that aborts in the retro phase seals through the abnormal-exit epilogue, and the epilogue assembles the digest again (cycles 2043, 2046 and 2055). `withRecordedClass` keeps the class fields of the harness record when the exact fingerprint did not change. A new exact fingerprint drops them, because the class belongs to the old failure.

**Count.** Only the layers `pipeline-code`, `harness` and `infra` get a class. A `task-code` failure keeps only its exact fingerprint, so cycle 2048 does not count. `class_recurrence` is 1 plus the number of other records in `.evolve/failure-digests/` with the same class.

Each class record carries `class_recorded_at`, the clock time of the retro completion. A record with a `class_recorded_at` before the last `resolved_at` of the class in `.evolve/resolved-fingerprints.json` does not count. Thus a resolved class starts again at 1. A file copy does not change the count, because the count does not read the file time.

**Route.** `recordFailureClass` gives the class recurrence and the disposition routing to `dispositionrouter.Decide`. The result goes to `class_route` and `class_route_reason`, and the layer goes to `class_layer`. The third failure of the class routes to the console with the reason `floor: recurrence 3 >= 3`. A console route stages one autofile intent through `dispositionrouter.StageIntent` (item `failure-class-<hash>`, priority class `correctness`). The boundary applier files that item, as for each other staged intent.

**Breaker.** The `identical-fingerprint` rule counts a digest by its class fingerprint when it has one, else by its exact fingerprint. An ack of either fingerprint removes the digest from the count.

**Worked example.** The real reasons are fixtures in `go/internal/core/testdata/failureclass/`.

| Cycle | Layer | Checks | Class recurrence | Route |
|---|---|---|---|---|
| 2043 | pipeline-code | `composed-floor` | 1 | queue |
| 2046 | pipeline-code | `composed-floor` | 2 | queue |
| 2048 | task-code | `composed-floor` | no class | none |
| 2055 | pipeline-code | `composed-floor` | 3 | console |
| 2054 | pipeline-code | `explanation-documentation` | 1 (other class) | queue |

**Rejected option.** A class key from the detailed check names (`size`, `apicover`, `coverage`, `ste-lint`) splits the three cycles into three classes. Each cycle hit another set of composed-floor checks. The class key uses two groups that `build_handoff_floor.go` names beside each other: `explanation-documentation`, the mandatory floor, and `composed-floor`, each other floor check. Thus one root cause gives one class.

**Limit.** The class is coarse. Two pipeline-code defects in one floor check share a class until an operator resolves the class. A malformed `resolved_at` is an error of the ack ledger, so the class record and the breaker each give a warning.

Pinned by `TestFailureClass_TheFloorShortCircuitCyclesShareOneClassAndTheThirdRoutesToTheConsole`, `TestFailureClass_TheExplanationFloorVariantIsADifferentClass`, `TestFailureClassKey_EachPartSplitsTheClass`, `TestFailureClass_AResolvedClassStartsCountingAgain` and `TestBlockerBreaker_OneClassWithDistinctExactFingerprintsHalts`.

## Acceptance coverage

`go/acs/cycle1440/predicates_test.go` exercises each seam by running the named tests
in the production packages. It never uses a source grep, because a source grep passes on a cosmetic edit
that normalizes nothing. `TestC1440_002_CarryoverRetirementWiredIntoPassCloseout` is
the test that actually gates §1. It drives `promoteInbox` end to end and asserts the
observable `state.json` mutation, because a seam whose only caller is a test is dead
code.
