# Retro-verdict FAIL: root cause, and a review of pipeline retry as an architecture

**Date:** 2026-08-27 · **Scope:** why the retro phase reports FAIL, and how retry is layered across the loop

---

## Part 1 — The retro verdict is a false FAIL in 92% of cases

### The finding

`retro.go` grades its own phase with a two-condition rule:

```go
verdict := core.VerdictPASS
if strings.TrimSpace(content) == "" || !hasFailureLesson(req.Workspace) {
    verdict = core.VerdictFAIL
}
```

and `hasFailureLesson` scans **the cycle workspace** for a file whose name **starts with
`failure-lesson`** and ends `.yaml`.

The retro persona is instructed to write something else, somewhere else:

> Output path: `.evolve/instincts/lessons/inst-LXXX-<slug>.yaml`

Two independent mismatches, either of which alone is fatal:

| | gate expects | persona writes |
|---|---|---|
| directory | `.evolve/runs/cycle-N/` (the workspace) | `.evolve/instincts/lessons/` (repo-level) |
| filename | `failure-lesson*.yaml` | `inst-LXXX-<slug>.yaml` |

**The phase is graded FAIL for not producing an artifact its own persona is instructed
never to produce.**

### The evidence

- `.evolve/instincts/lessons/` holds **593** lesson files, including ones for the exact
  cycles graded FAIL: `inst-L1572a-…`, `inst-L1574a/b/c-…`, `inst-L1575a-…`.
- Exactly **one** file named `failure-lesson*` exists anywhere in the tree
  (`cycle-1571/failure-lesson-cycle1571.yaml`) — a lone historical outlier.
- Across all cycles on disk: **238 have a retro verdict of FAIL. 220 of those (92%)**
  wrote a substantial retrospective (>500 bytes) and no workspace `failure-lesson*.yaml`.

Wave 3/4 in detail:

| cycle | retrospective-report.md | disposition.json | workspace lesson | retro verdict |
|---|---|---|---|---|
| 1572 | 8,442 B | yes | none | FAIL |
| 1573 | 5,323 B | yes | none | FAIL |
| 1574 | 9,185 B | yes | none | FAIL |
| 1575 | 7,192 B | yes | none | FAIL |
| 1576 | **0 B** | **no** | none | FAIL (genuine) |

Cycle-1576 is the one real failure in the set — an empty retrospective. The other four did
the work, wrote the disposition, produced lessons, and were graded FAIL anyway.

The irony worth recording: cycle-1575's lesson is
`inst-L1575a-fleet-partition-must-bind-goal-eligibility-before-minting-lanes.yaml` — the
system correctly diagnosed the very partition defect that stranded that lane, and its
retro was marked FAIL for doing so.

### Why this matters beyond cosmetics

1. **It destroys the signal it was built to provide.** A real retro failure (1576) is
   indistinguishable from the 92% noise. The one verdict that would tell an operator "the
   failure analysis itself broke" is permanently pinned to FAIL.
2. **It contaminates routing.** `decideAfterRetro` is driven by the retro verdict. Every
   downstream decision — failure-adapter action, floor evaluation, and the ADR-0092 repair
   branch — takes its input from a value that is FAIL almost unconditionally.
3. **It inflates failure counters.** Retro FAILs feed the failure history the
   consecutive-failure breaker and non-progress dossier read.
4. **It is the same class as the persona-strip lobotomy (#434).** There, the dispatched
   prompt was graded against a source persona nobody dispatched. Here, the phase is graded
   against an artifact contract nobody writes. In both cases the *gate's model of the
   world* diverged from the world, and the gate was believed.

### A masked second defect — and a correction to this document's own recommendation

Checking the consequence of fixing Part 1 turned up a second defect that the first one is
currently HIDING. `decideAfterRetro` opens with:

```go
// retro PASS → ship; no failureadapter consultation, no floor (nothing failed).
if retroVerdict == VerdictPASS {
    return o.recoveryTarget(PhaseRetro, VerdictPASS, PhaseShip), nil, "retro-recovered: ship", nil
}
```

Two things are wrong with this, and both are invisible today because retro almost never
returns PASS:

1. **The comment's premise is false.** Retro runs ONLY when the previous verdict was
   FAIL/WARN (`retro.go`: "previous verdict != FAIL/WARN → SKIPPED"). "Nothing failed" is
   never true on this path — the audit failed, which is why retro ran at all.
2. **It is a category error.** Retro PASS means *"the post-mortem deliverable is complete"*
   — a non-empty retrospective plus a lesson file. It says nothing about whether the
   underlying defect was fixed. The routing reads it as *"the cycle recovered"* and sends
   it to ship.

The ship gate does fail closed — `CodeAuditBindingVerdictFail`, "auditor explicitly
rejected this build" — so nothing rejected can actually ship. The backstop holds. But the
routing is still wrong in two ways that matter:

- a failed cycle would be routed to a ship that must refuse it, converting a clean
  terminal disposition into a ShipError plus L5 recovery — wasted work and a confusing
  trail;
- **it would bypass L4 entirely.** The ADR-0092 repair branch lives BELOW this early
  return, so a correctly-PASSing retro would skip the floor, the bookkeeping regrade, and
  the audit repair.

**Correction to recommendation 1 below.** This document originally said "fix the retro gate
first". Followed literally, that would activate a dormant category error and disable the
in-cycle repair loop in the same change. The two defects must be fixed TOGETHER, and the
routing half is the one to fix first:

- `decideAfterRetro` must stop treating retro's deliverable-quality verdict as a recovery
  signal. A retro PASS on a FAIL cycle should fall through to the SAME floor → regrade →
  repair → adapter ladder as a retro FAIL; the retro verdict should gate only whether the
  post-mortem itself needs re-dispatch.
- Only then is it safe to make the retro gate capable of passing.

This is a textbook masked-defect pair: defect 1 (the gate cannot pass) has been suppressing
defect 2 (PASS is routed as recovery) for as long as both have existed. Fixing the visible
one alone would have shipped the hidden one into production.

### Fix shape (not yet implemented)

The gate is the wrong half to keep. The persona's convention (`inst-<id>-<slug>.yaml` in a
shared, cross-cycle lessons directory) is the one that is actually load-bearing — those 593
files are read back into future agents' `instinctSummary`. Recommended:

- `hasFailureLesson` resolves lessons the way the persona writes them: check
  `.evolve/instincts/lessons/` for a lesson **whose id names this cycle** (`inst-L<cycle>*`),
  which also makes the check cycle-scoped rather than "any lesson anywhere".
- Keep accepting the legacy workspace `failure-lesson*.yaml` shape so cycle-1571-era
  artifacts still pass.
- Add a single-source test that the persona's documented output path and the gate's search
  path are the same string — the drift class this whole finding is an instance of.
- Pin cycle-1576's shape (empty retrospective ⇒ genuine FAIL) so the fix cannot turn the
  gate into a rubber stamp.

---

## Part 2 — Pipeline retry, as an architecture

Retry is not one mechanism here; it is **nine layers**, built at different times for
different failure classes, with no shared budget and no shared vocabulary.

### The layer map (as built)

| # | Layer | Trigger | Mechanism | Bound | State |
|---|---|---|---|---|---|
| L0 | transport | artifact timeout, launch failure | bridge extends / re-launch | `max_extends`, boot timeout | live |
| L1 | deliverable | contract violation (wrong path, malformed) | correction ladder (ADR-0045 I2); rung 1 salvages agent-free | 3 rungs | live |
| L2 | deterministic gate | a listed gate phase FAILs | graduated remediation: re-dispatch builder, re-run the same gate | `remediation_rounds` (1), `remediable_phases` (**coverage-gate only**) | live |
| L3 | bookkeeping | audit FAIL explained *only* by bookkeeping gates | retro→audit regrade | once per cycle | live |
| L4 | judgment | task-level audit rejection | ADR-0092 in-cycle repair → tdd→build→audit | 2, durable counter | live (#507) |
| L5 | ship | ShipError | `recoverFromShipError`; debugger `RESHIP` / `RERUN_PHASE` | `maxRecoveryDepth` | live |
| L6 | cycle | cycle FAIL with preserved worktree | continuation registry + snapshot re-seed | registry lifecycle | live |
| L7 | **wave** | a lane cannot produce (ineligible scope, dead assignment) | — | — | **GAP** |
| L8 | batch | N consecutive failures / identical fingerprints | blocker breaker → HALT | ceiling 3 | live |

### The organising principle already in the code

`policy.go` states it plainly, and it is the right principle:

> RemediablePhases lists the **DETERMINISTIC** gate phases eligible for graduated
> remediation. **Judgment phases (audit, adversarial-review, premise-challenge) must never
> be listed** — remediation is for mechanical, prescribed defects only.

That is a real architectural line: *a mechanical defect may be fixed and the same gate
re-run; a judgment may not be re-rolled until it agrees.* Re-running a judge until it
relents is how a gate becomes a rubber stamp.

**ADR-0092 sits on the wrong side of that line and must justify itself.** It retries after
a *judgment* phase. The distinction it relies on — and which should be stated explicitly in
the ADR rather than implied — is **re-earn vs. override**:

- **Override** (forbidden): re-run the same judge on the same artifact until the verdict
  flips. Nothing changed but the dice.
- **Re-earn** (what L4 does): the cycle goes back through **tdd → build**, producing a
  *different tree*, and the audit is run again on that new tree. The rejection is not
  overturned; it is made obsolete by new work.

L2 is nearer the danger than L4 is, because it re-runs *the same gate* on a builder patch;
it is safe only because the gate is deterministic. L4 re-runs a judge, but never on the
same input. Both are defensible; neither is defensible without stating which side of the
line it is on.

### Gaps, ranked

**G1 — L7 (wave) does not exist.** When a lane is assigned a scope it cannot legally work,
nothing reassigns it. Cycle-1575 is the proof: `fleet.Partition` handed it exactly one
item, the goal excluded that item, scout and triage both correctly refused, and the lane
then burned seven phases and closed retro/FAIL. Every lower layer worked correctly; the
missing layer is the one that should have said *"this lane has no work, give it different
work."* The system's own retro named this
(`inst-L1575a-fleet-partition-must-bind-goal-eligibility-before-minting-lanes`).

**G2 — no shared retry budget.** Each layer bounds itself independently, so worst-case cost
composes multiplicatively: L2 rounds × L4 attempts × L6 continuations × L0 extends. Nothing
computes "this cycle has spent N of its retry budget". A cycle can be expensive in a way no
single bound predicts.

**G3 — L2 covers one gate.** `remediable_phases` defaults to `["coverage-gate"]`, yet the
same *mechanical, prescribed* shape applies to EGPS/ACS red counts, repo-contract scanner
findings, and type-safety audit output. Wave-3 cycle-1574 died on
`red_count=1 [record_absent_from_inbox_root_exactly_once]` — a deterministic, prescribed
defect that L2 would have been the right layer for, had that gate been listed.

**G4 — retry is invisible in telemetry.** No artifact answers "how much of this cycle was
retry?" `attempt_count` exists per phase, `AuditRepairAttempts` is new, remediation rounds
are logged — but nothing aggregates them. The economics argument in
`cyclerun_remediate.go` (~1–2M tokens to remediate vs ~12.5M to discard a cycle) is exactly
the kind of claim that should be measured continuously, and currently is not.

**G5 — the layers share no vocabulary.** L2 says "remediation round", L3 "regrade", L4
"repair attempt", L5 "recovery depth", L6 "continuation", L0 "extend". An operator reading a
failing cycle cannot ask one question and get one answer.

### Recommended direction

1. **Fix the retro ROUTING before the retro gate.** See "A masked second defect" above for
   why repairing the gate alone is unsafe. Action: make `decideAfterRetro` fall through the
   normal floor → regrade → repair → adapter ladder on a retro PASS, then make
   `hasFailureLesson` capable of passing.
2. **Close G1 at the partitioner, not with a new retry layer.** The right fix is
   goal-aware partitioning plus a loud launch-time failure when a directive names ids the
   resolved lane set cannot contain — already filed as
   `goal-text-has-no-selection-authority` and `spine-dispatches-build-after-empty-triage`.
   A wave-level retry layer would be *compensating* for a scheduling defect; better not to
   need it.
3. **Give the ladder one budget and one vocabulary (G2/G5).** A single `retry` block in
   `policy.json` with per-layer caps and one cycle-wide ceiling, and one
   `retry-ledger.json` per cycle recording every attempt at every layer with its cost. This
   is the smallest change that makes the economics auditable.
4. **Widen L2 deliberately (G3)** to the deterministic gates that meet the "mechanical and
   prescribed" bar — EGPS/ACS and repo-contract — and leave every judgment phase off the
   list, permanently.
5. **State the re-earn/override principle in ADR-0092** and make it the explicit test any
   future retry layer must pass.

### What NOT to do

- Do not make judgment phases remediable. The `policy.go` comment is correct and should
  become a guard test, not just a comment.
- Do not raise any bound before G4 exists. Every current bound was picked without
  measurement; raising one blind trades a known cost for an unknown one.
