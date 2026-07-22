# Evolve-Loop: Lessons & Resolutions — the July 2026 Pipeline-Hardening Campaign

> **Landing path:** `knowledge-base/research/lessons-and-resolutions-2026-07.md`
> (same home as the canonical cycles 215–231 retrospective).
> **Scope:** cycles ~850–1033, releases v22.3.0 → v22.7.0, 2026-07-14 → 2026-07-22.
> **Audience:** anyone (human or agent) who needs to understand *why the pipeline
> is built the way it is now* — every gate and floor below exists because a real
> failure demanded it.

---

## 1. Executive summary

Over eight days the loop's batch pass rate went from **~65% (24 PASS / 13 FAIL,
batches 1–3) toward a projected ~90%**, not by making the auditor more lenient
but by eliminating six families of *false* failures while keeping every *correct*
rejection intact. The campaign closed with five releases (v22.3.0–v22.7.0) and a
standing rule set that changed how the operator and the loop divide work.

The single most important insight, inherited from the June retrospective
(cycles 215–231) and confirmed by everything since:

> **The pipeline is a distributed system** — LLM agents × Go kernel × three git
> trees × replicated configs — **and every recurring defect was a coherence
> failure between replicas of hand-authored belief.** The fixes that stuck were
> invariants and deterministic floors, never prompt nudges. 238 prompt-injected
> lessons did not stop a single recurrence; one deterministic gate did.

---

## 2. The defect families, and how each was closed

### F1 — Forged verdicts & verdict incoherence (cycles 603, 862–899, 921–932)

**Symptom.** Cycles with a green audit report and ship-eligible ACS verdict were
recorded as FAIL; the loop re-selected the same task forever (the 862→899
token-burn storm rebuilt one feature five times and discarded it five times).

**Root cause (two layers).**
1. The integration-tier gate (`go test -race -tags integration`) **false-REDs
   under fleet contention** — `internal/core`'s integration tests drive full
   orchestrators over real git and cannot share a machine with live lanes. The
   suite was verified green in isolation *and* in the preserved failing
   worktree.
2. `detectVerdictIncoherence` **never populated `SubstantiveError`**, so a
   diagnosed task-level FAIL looked identical to a forged verdict → ADR-0072
   HALT.

**Wrong turns — recorded deliberately.** Five successive fixes targeted
non-causes (settle-window band-aid, pane-removal, late-write race, size-stability
gate, teardown routing). Three LLM retrospectives repeated a plausible
"stall→teardown" narrative. The truth was in one place the whole time: the
`state.json` defect strings were **byte-identical across four cycles** — the
CI-parity gate's own diagnostic template.

**Resolution.** `8e2afef0` + `3c5ed711` (PR #340) + `f09ecfc4`:
`audit-fail-reason.json` persisted on every floor failure and read back into
`SubstantiveError` (diagnosed FAIL = continue; unexplained FAIL still halts —
the true forgery signature is preserved); integration tier scoped to touched
packages and skipping env-exclusive ones; offender lines line-anchored with the
full log preserved.

**Lesson.** *Fingerprint the deterministic defect strings before believing any
narrative — including your own previous one.* Diagnostics must be persisted as
artifacts, not derived at failure time.

### F2 — The shared-state disease (cycles 992, 994, 995, 999, 1000, 1001)

**Symptom.** Six cycles in two batches false-failed on state corruption:
carryover todos vanished, `state.json` lineage severed, one cycle destroyed
another's committed decisions.

**Root cause.** `state.json` is a **symlink** in worktrees; a hand-rolled
temp+rename writer in `cmd_carryover.go` renamed *over the symlink*, severing it
and forking the lineage. Separately, read-modify-write races between lanes lost
writes silently. The adversarial reviewer had flagged the RMW race as MEDIUM in
cycle-992 — *before* it destroyed data in 1001 — and nothing escalated the
warning into the queue.

**Resolution (console-first).** PR #345 `9aaa4389`: symlink write-through
(`resolveWriteTarget`), dual-lineage CAS floor (`statemapRevision` owned by
statemap; `stateRevision` compared-never-bumped as storage's exclusive audit
trail), canonical-resolved locking, the severing writer migrated to the safe
API, quarantine for corrupt state, and a shrink tripwire on carryover todos.
Recovery proved every layer live: cycle-1000's lost 136-decision document was
re-applied through the fixed binary (carryover todos 144 → 8).

**Lesson.** *A reviewer warning without a routing decision is a lesson lost* —
this incident is a direct motivator for the failure-disposition router. And:
never hand-roll a second writer for state that already has one.

### F3 — Cross-lane false aborts from shared main-tree writes (cycle 967 and 3 operator incidents)

**Symptom.** A lane's tree-diff guard aborted an innocent cycle because *someone
else* (another lane's phase-mint, or the operator's console session) wrote a
tracked path in the shared main tree.

**Resolution.** ADR-0073 mint registry (PR #344 `5fdf1838`): phases register
their minted files in `.evolve/active-mints.json` before persisting; the guard
exempts exactly the registered, TTL-fresh, content-verified paths. Two security
review passes settled an important principle: with sandboxing off, **no on-disk
anchor is authenticatable — the achievable property is clamp parity, not
provenance** (accepted residual, documented).

**Lesson (operator side).** Three cycles (1011, 1023, 1027) were still killed by
*my* mid-batch console writes — including a revert. The discipline is absolute:
**no tracked-path main-tree writes while a batch runs; only `.evolve/inbox` is
safe; all code work in worktree branches; landings at batch boundaries.** Now a
standing rule.

### F4 — Builders handing off known-broken work (cycles 983, 1008, 1009)

**Symptom.** Build phases delivered non-compiling or red-testing trees; the
failure surfaced cycles' worth of tokens later at audit. Smoking gun:
cycle-1008's `build-selfcheck.json` **recorded `./cmd/evolve` failing at
handoff** — the builder ran the check, saw red, and handed off anyway.

**Resolution.** PR #347 `1816d553` — **build handoff floor**: a deterministic
`BuildFloorReviewer` runs first in the deliverable-review chain, diffs against
the worktree *base* SHA (a HEAD diff is empty after the builder commits —
caught in review as a vacuous-approve), runs changed-package tests + coverage-
backed API-coverage checks, and REJECTS a red build into the in-phase
correction ladder. Converts a ~12.5M-token FAIL cycle into a ~1–2M in-phase fix.

**Lesson.** *Deterministic checks belong at the earliest seam that can act on
them* — shifting the gate from post-audit to build handoff changed its cost by
an order of magnitude. (Operator directive that drove it: "shift deterministic
gates to the front as part of build-phase verification.")

### F5 — All-or-nothing waste on minor errors (cycles 1019, 1020)

**Symptom.** A correct implementation with one minor gate failure was discarded
wholesale; the next cycle re-implemented from scratch and failed identically.
Cycle-1019's discarded work was salvageable in full (landed later as PR #346).

**Resolution.** PR #347 — **graduated remediation**: a FAIL from a configured
remediable phase (default: coverage-gate, one round) dispatches the builder to
fix the specific gate finding and re-runs the *same* gate for a fresh verdict,
with full provenance (`CycleResult.Remediations`) and judgment phases
(audit/adversarial) hard-denied from remediation — correctness gates iterate,
judgment verdicts never do. Plus **S5 task quarantine** (PR #346): a task that
keeps drawing doomed same-family attempts is quarantined instead of re-drawn.

**Lesson.** *Distinguish "wrong implementation" from "right implementation,
minor defect"* — the verdict system must be able to say both. (Operator framing:
"we should not throw away all the effort for minor errors.")

### F6 — Quota-exhaustion storms & model-wording drift (cycles 877–911)

**Symptom.** A model's quota-message wording changed; the detection regex
silently stopped matching; the loop relaunched into the same wall in a livelock
(34 cycles, no ship).

**Resolution.** v22.4.0 `4dc3d4c9`: per-model exhaustion regexes (#328)
validated against real pane captures, goal-stall breaker (#330), tier-fallback
(#331), persistence gate (#332).

**Lesson.** *Validate detection patterns against the real surface, and prefer
false-positive-safe designs* — a missed quota signal costs a livelock; a
spurious one costs a pause. Asymmetry decides the design.

### F7 — Work lost after PASS, and inert shipped APIs (cycles 948, 962, 975)

**Symptom.** Cycle-948 recorded PASS but its commit never landed on main
(fleet-ship race) — the work was neither landed nor re-queued, because item
consumption fires on *pick*, not on *land*. Cycle-962 shipped an API with zero
callers (flagged only by coverage tooling). Cycle-975 was *correctly* rejected
for the same inert-API defect.

**Resolution / status.** Landed: the 975 arc completed properly (composer +
live ship-path caller, closing the inert-API instance). Queued at top band:
`ship-landed-verification-floor` 0.96 (PASS ⇒ reachable-from-main check;
item consumption transactional with landing).

**Lesson.** *PASS ≠ landed* — verify with `git cherry origin/main <lane-branch>`.
*A new exported API with no callers is an inert ship*; the coverage
uncovered-flag is the early symptom, not noise.

### F8 — Silent gate refusals (cycles 1012, 1022, 1028)

**Symptom.** A gate refused to ship and said nothing actionable (silent EGPS
refusal; a correct build discarded over a label-string mismatch).

**Resolution.** PR #348: ship-eligibility decision artifacts (every gate refusal
writes its reason) + top-N task-id injection as plumbing, not prose; topngate
label drift downgraded to a loud advisory (the committed set binds, a mislabel
no longer discards correct work). Live proof: cycle-1028's FAIL was diagnosed
from `audit-fail-reason.json` in seconds — "EGPS: red_count=1", a legit
rejection, correctly left alone.

**Lesson.** *Every "missing artifact" or silent refusal is a diagnosability
defect in the writer*, not just an inconvenience for the reader.

---

## 3. What we deliberately did NOT fix

Correct rejections are the pipeline working. The campaign preserved them:

- **975** — inert-API refusal (the gate was right; the arc later landed properly).
- **991** — premise-challenge BLOCK.
- **1028** — EGPS red_count=1, honest red.
- Coverage-floor and archival-contract FAILs in batch-2 (983, 984, 992, …) were
  precise task-level catches, zero false-REDs.

A "fix" that would have raised the pass rate by weakening these would have been
gaming the metric. The target state is: **all remaining FAILs are ones we want.**

---

## 4. Meta-lessons (process — the ones that transfer beyond this codebase)

1. **Fingerprint before narrative.** Deterministic defect strings (state.json,
   fail-reason artifacts) outrank every plausible story, including three
   consecutive LLM retrospectives and your own previous diagnosis. Five refuted
   hypotheses in F1 all came from inference; the evidence was one grep away.
2. **Unit-green ≠ live-green.** Every floor/gate needs a *wiring proof* on the
   live path — the checkpoint package that was inert in production (blank
   import only in tests) passed four audits. Review the wiring, not just the
   leaf logic.
3. **PASS ≠ landed.** A recorded verdict is not a merge on main. Verify
   reachability; make consumption transactional with landing.
4. **The lesson-to-action gap is the meta-defect.** The loop *noticed* every
   recurrence (retros wrote "7th occurrence" chains) but noticing lived in
   advisory channels with no write access to the priority queue. Deterministic
   ledgers and auto-filing close it; prompt nudges provably do not.
5. **Console-first for pipeline-integrity defects.** Queueing a pipeline fix
   into a broken pipeline cannot work — the fixing lane rides the broken
   machinery. Now a standing rule, and the routing test at the heart of the
   failure-disposition design.
6. **Salvage before requeue.** Preserved worktrees from false-failed cycles held
   complete, correct implementations (1019, 948, 876/897/898). Discarding them
   and rebuilding is the single most expensive habit the loop had.
7. **Mid-batch write discipline (operator).** Three innocent cycles died to
   console writes in the shared tree — including a revert. Boundaries exist for
   humans too.
8. **Asymmetric detection design.** For exhaustion/quota signals, a false
   positive pauses; a false negative livelocks. Design for the cheap failure.
9. **New packages must enroll in coverage enforcement at creation.** The same
   apicover parity gap recurred five times in one week across console and lane
   ships alike.
10. **Missing artifacts are writer defects.** If a failure mode can end with
    "file not found", the writer must emit the *reason* into an artifact in
    every branch — the reader's grep is not a diagnosis strategy.

---

## 5. Standing mechanisms built by this campaign

| Mechanism | Prevents | Landed |
|---|---|---|
| ADR-0072 system-failure floor (HALT on forged verdict, rc 4) | verdict-forgery livelocks | `d531fc32`, v22.3.0 |
| `audit-fail-reason.json` → `SubstantiveError` | diagnosed FAILs halting as forgeries | `8e2afef0`, v22.4.2 |
| Integration-tier scoping (touched pkgs, env-exclusive skip) | contention false-REDs | `f09ecfc4`, v22.4.2 |
| Statemap symlink write-through + dual-lineage CAS | shared-state severing / lost writes | PR #345, v22.7.0 |
| Mint registry + guard exemption (ADR-0073) | cross-lane false aborts | PR #344, v22.5.0 |
| Build handoff floor (`BuildFloorReviewer`) | known-broken handoffs | PR #347, v22.7.0 |
| Graduated remediation + provenance | all-or-nothing waste on minor defects | PR #347, v22.7.0 |
| S5 task quarantine | poison-task re-draws | PR #346, v22.7.0 |
| Ship decision artifacts + top-N id plumbing | silent refusals, label-drift discards | PR #348 |
| Per-model exhaustion regexes + tier fallback | quota livelocks | v22.4.0 |
| ADR-0074 plan-time routing gate + claim backstop (typed routing authority; revives the S5 claim→counter→quarantine chain dead since inbox-mover.sh's deletion) | control-plane task burns in lanes | PR #349 `f28dedf0`, live-verified exit-3 refusal |
| **Failure-disposition router** (assembler → retro disposition contract → floors/router → boundary applier) | undispositioned FAILs; closes the lesson-to-action gap | S1+S2 built by cycle-1034 (salvaged, landing); S3+S4 queued 0.96 |

## 6. Outcomes

| Batch (post-fix era) | Result | Notes |
|---|---|---|
| Batch 1 (974–979) | 4 PASS / 1 FAIL | the FAIL (975) was a correct rejection → v22.5.0 |
| Batch 2 (980–99x) | 9 PASS / 6 FAIL | zero false-REDs; every FAIL a precise task-level catch → v22.6.0 |
| Batch 3 (…–1009) | 7 PASS / 7 FAIL | +1 operator-aborted; produced the 13-FAIL cluster analysis |
| Batch 4 (…–1027) | 10 PASS / 4 FAIL | 3 of the 4 were operator mid-batch pollution → v22.7.0 |
| Batch 5 (1028–1043) | 6 PASS / 7 FAIL | six of seven FAILs = ONE control-plane routing class (→ ADR-0074; all six worktrees salvaged, ~3 work items recovered whole); one operator-collateral empty draw; zero pipeline false-REDs |

## 7. Source index

- `knowledge-base/research/campaign-retrospective-cycles-215-231-2026-06-06.md` — the unifying diagnosis
- `knowledge-base/research/fail-analysis-pass-rate-2026-07-21.md` — the 13-FAIL clustering
- `knowledge-base/research/token-usage-history-2026-07-20.md` — token-efficiency baseline
- `docs/architecture/adr/0072-*` (system-failure policy) · `0073-*` (mint registry)
- PRs #337–#348; releases v22.3.0 `1961925b` · v22.4.2 `94aa6629` · v22.5.0 `0fc1c332` · v22.6.0 `59950d59` · v22.7.0 `a312a60f`
- `.evolve/instincts/lessons/` — per-cycle lesson corpus (267+ entries; 59% generic floor noise — see chronicle campaign for the classification)
