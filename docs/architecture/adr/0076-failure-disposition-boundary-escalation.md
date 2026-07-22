# ADR-0076 — Failure-disposition routing floors and boundary escalation

- **Status:** Accepted (shadow-first)
- **Date:** 2026-07-23
- **Cycle:** 1062 (`chronicle-s6-escalation-boundary`, superseded into `failure-disposition-router` S3+S4)
- **Supersedes/extends:** the recurrence ledger (cycle-661), `internal/retrofile` (cycle-657)

## Context

The loop already NOTICED recurrence — retro closeouts wrote "6th occurrence,
confidence 0.97" chains — but noticing lived in an advisory channel with no
write access to the priority queue. Two mechanisms existed to close that loop
and **both were inert**:

- `recurrence.Escalator` / `recurrence.Autofiler` (cycle-661): interfaces with
  zero production implementers — only test fakes satisfied them.
- `internal/retrofile` (cycle-657): a complete inbox filer with **zero callers**
  anywhere outside its own package.

This is the `lesson_to_action_gap` meta-defect in its third instance: the
diagnosis lands, the queue never changes.

The blocker was not the formula but the *write moment*. Escalation is decided
mid-cycle, while fleet lanes are in flight. An inbox write at that moment races
`inboxmover.Claim`'s `os.Rename` on a claimed item and resurrects it into
double work across lanes — a race the chronicle-s6 spec pinned verbatim.

## Decision

Split the mechanism at the race boundary.

**S3 — `go/internal/dispositionrouter` (decide + stage, never write the inbox).**

- `Decide(preClass, recurrence, llmRoute) Decision` applies two deterministic
  floors that force `console`: the `guard-abort` pre-class (a severed statemap
  is pipeline machinery, never a lane task) and `recurrence >= 3` (a defect that
  survived two fixes is no longer lane-sized). The advisory LLM route may RAISE
  `queue → console`; it may **never lower** a forced console. An empty or
  unknown advisory route is a no-op.
- `StageIntent` appends one JSONL record to
  `.evolve/escalations/pending-actions.jsonl` under the shared file lock. This
  package never touches `.evolve/inbox/`.

Named `dispositionrouter`, not `router`: `internal/router` is the unrelated
phase/model dispatch router, and conflating two routers in one package would be
a namespace collision, not reuse.

**S4 — `recurrence.ApplyBoundary` (the only inbox writer on this path).**

Runs at the loop's per-iteration boundary (`cmd/evolve/cmd_loop.go`, in all
three dispatch branches: wave, pool, sequential) — after dispatch returns and
before the next iteration is planned, i.e. the one moment no lane is in flight.
It holds the inbox lock for the whole read-modify-write and guarantees:

| Property | Mechanism |
|---|---|
| Idempotent per cycle | `applied-stamp.json` beside the staging file; a key already stamped with this cycle is skipped |
| Never lowers a weight | a computed target `<=` the item's current weight is skipped, not written |
| Claimed items untouched | an item under `inbox/processing/` is reported `Skipped`, never bumped and never re-filed |
| Shadow is report-only | the stage plans everything, writes the report artifact, mutates nothing |
| Autofile is not a second filer | the autofile path calls `retrofile.FileActions` — its **first production caller** |

**Stage and formula are config, not flags.** `.evolve/policy.json`'s
`failure_disposition` block carries `stage` / `threshold` / `step` / `cap`;
compiled Go defaults apply when absent, and `stage` projects from the existing
`chronicle.escalation` stage (default `shadow`) so the same word is not spelled
twice. No new `EVOLVE_*` flag gates this feature.

## Consequences

- Recurrence finally reaches the queue: a recurring pattern's open item gets its
  weight bumped, or an orphan pattern is filed as real work.
- `retrofile` and the `Escalator`/`Autofiler` seams stop being dead API.
- Default stage is `shadow` — the artifact
  (`.evolve/escalation-apply-report.json`) is written every iteration, and no
  inbox mutation happens until an operator sets `stage: "enforce"`.
- The boundary call is best-effort: a failure WARNs and never breaks dispatch
  (`never_stop_queue_inject_inbox`).
- S3 currently exposes `Decide`/`StageIntent` as a library; the classifier call
  site that *produces* intents (wiring `failure_digest.go` /
  `disposition_gate.go` output into `StageIntent`) remains open work.

## Verification

`go/acs/cycle1062/predicates_test.go` — 10/10 behavioural predicates, all
driving the production API against `t.TempDir()` fixtures. Predicate 010 proves
the loop call site by EXECUTING
`cmd/evolve: TestRunLoop_EscalatesAtIterationBoundary` (which drives the real
`runLoop` and asserts the weight actually moved), not by grepping for the call.
