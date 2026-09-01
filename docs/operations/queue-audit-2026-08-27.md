# Queue Audit — carryoverTodos + inbox
**Date:** 2026-08-27 · **Scope:** 764 `carryoverTodos` (`.evolve/state.json`) + 98 live inbox records

## Headline

`carryoverTodos` is **not a backlog**. It is an observation log that was wired to a work queue, and it
has no consumption lifecycle: nothing ever marks an entry done. Entries only accumulate, increment
`cycles_unpicked`, and have their TTL rolled forward. The inbox is the real curated queue.

Evidence for "no lifecycle": of 640 non-telemetry carryover ids, **4** are backed by a live inbox
record and **1** appears in `inbox/consumed/`. The two queues are almost entirely disjoint, so
finishing work never removes a carryover entry.

## Population — three different things in one list

| class | count | what it actually is |
|---|---|---|
| `cycle-N-failed-PHASE` | 124 | failure telemetry (action = a stack trace), not a task |
| 32-char hex id | 144 | auditor `PRESCRIPTION:` items from defect ledgers — real content, opaque id |
| `todo-*` / named | 496 | the genuine backlog |

Telemetry records by phase:

| phase | count |
|---|---|
| audit | 116 |
| build | 8 |

## Why the queue is 764 — corrected

An earlier draft of this audit claimed the TTL was "refreshed rather than enforced". **That was wrong**,
and the data disproves it. Expiry correlates monotonically with `first_seen_cycle` — cycle 1268 expires
Sept 3, cycle 1424 expires Sept 9, cycle 1572 expires Sept 25 — so each entry is stamped exactly once at
creation and never re-stamped. The mechanism is sound end to end:

- `mergeCarryoverTodos` (`failure_learning.go:744`) is keep-first: an existing id is never overwritten.
- `failurelog.PruneExpiredCarryoverTodos` is real and **is wired**, at `cmd_loop.go:236`, every loop start.

The queue is 764 because **inflow x TTL** says so. `CodeBuildFail` / `CodeAuditFail` carry a **30-day**
age-out (`classifications.go:93`), and audit failures are the dominant inflow. Nothing is expired yet
simply because nothing has aged out *yet*: a large fraction falls due over the next four weeks.

That makes the real hazard the opposite of bloat. **Unpicked real work is on a 30-day fuse.** The 33
entries at >=100 boots unpicked will expire without ever having been worked, silently, with no record
that they existed. A queue that both overflows and forgets is worse than one that only overflows.

One genuine defect: a single record, `cycle-1063-failed-build`, expires **2058-03-30**. It is classified
`IntentRejected`, whose age-out is `999999999` seconds — 31.7 years, a deliberate "effectively never"
sentinel (`classifications.go:92`). A failure record holding a slot in the *work queue* for 32 years is
the sentinel meeting the wrong array.

Age evidence — and a correction to what the counter means:

`cycles_unpicked` does **not** count cycles, and it is not decline evidence. It is
incremented once per **loop boot** by `failurelog.IncrementCarryoverUnpicked`
(`cmd_loop.go:250`), roughly twice on self-heal boots. A second contradiction worth
recording: the field's own struct comment in `cyclestate/state.go` calls it
"Deprecated (ADR-0072 S5) ... a dead field ... only ever written as 0", and current Go
does only ever write 0 there — but a different package increments it on disk every boot.
The comment is stale; the counter is live. Read the numbers below as **boots survived**.

| never picked across | items |
|---|---|
| >= 25 boots | 531 |
| >= 50 boots | 359 |
| >= 75 boots | 206 |
| >= 100 boots | 33 |

## The finding that explains wave 4

Wave 4's goal text named `phase-stub-shape-rule-at-ship-staging` as the first priority. That work
exists in the queue **only** as `dfdf2ec9c90444cffc4cfa16b0bd49f1b`:

> PRESCRIPTION: Class fix: replace the per-name .gitignore phase-stub list with a shape rule so any
> phase.json w...

It has no human-readable id, so no operator directive could ever have matched it — independently of
the goal-blind partitioner. Same for `untrack-regenerated-coverage-artifacts`, which exists only as
`d96d9d97eb92a5b79098eb2581aed75fe` and `daecbd33c574933aa8e9af9a0aee58efc`.

**144 of the highest-value items in the queue are addressable only by content hash, and all 144 are
stamped `priority: high`** — which also destroys prioritization, since "everything is high" is the
same as "nothing is".

## What is safe to drop, and what is not

**Drop outright — 124 telemetry records.** They are not tasks. Their content is a
captured stack trace, several referencing `/tmp/p/.evolve` (a test fixture path). The underlying
incidents are preserved in `docs/incidents/` and in the per-cycle run directories, so dropping loses
nothing.

**Do NOT drop on age alone.** The tempting cut — "declined 50+ cycles, therefore low value" — is
wrong here, and this audit found the reason. Selection is goal-blind: `fleet.Partition(todos, n)`
takes only a todo list and a lane count. An item's age measures how many loop boots it
survived without a blind partitioner drawing it, not how little it is worth. Cycle-1575 is
the proof: it was handed exactly one item, the one the goal excluded, and stalled a full
lane. Age-based pruning would delete work the selector never fairly considered.

**Harvest instead.** Of 220 entries under 25 cycles old, ~81 are already represented in the
inbox corpus, leaving roughly **139 genuine promotions**: give each a readable slug, a real weight,
and an inbox record with a consumption lifecycle.

## Inbox health (98 live)

The inbox is in good shape and is the model the other queue should follow: **zero** overlap between
live records and the 99 consumed / 5 processed records — the lifecycle works.

| weight band | count |
|---|---|
| 0.70-0.84 | 40 |
| >=0.85 (pipeline-reserved) | 28 |
| <0.70 | 22 |
| unweighted | 8 |

It is 98 rather than the expected <30 because it accumulates faster than waves consume it,
not because it is polluted.

## Recommendation

1. **Delete the 124 telemetry records** from `carryoverTodos`. Route future
   `cycle-N-failed-*` records to a failure log, never to the work queue.
2. **Give the 144 prescriptions readable ids** and real weights, then move them to
   the inbox. This is the highest-value action: it contains the CRITICALs that two waves failed to
   reach.
3. **Enforce the TTL that already exists.** Expiry is computed and then ignored; making it binding
   bounds the queue without a policy change.
4. **Fix the selector before pruning the backlog.** Until partition can honor an include/exclude set,
   `cycles_unpicked` is not a value signal, and any age-based prune destroys evidence.

Sequencing note: `state.json` is read by the live loop, so all mutations wait for a wave boundary.
