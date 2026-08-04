# Contract-block CLI escalation: the fix whose delay was the finding
**Period:** 2026-07-29 → 2026-08-05 (class first seen cycles 1171/1172; identity fix landed 2026-08-04) · **Status:** shipped (cycle-1294 vet follow-up open)
**Primary artifacts:** `docs/architecture/contract-block-cli-escalation.md` · `go/internal/core/contract_escalation.go` + `cyclerun_review.go` · PR #390 (`32671641`) · ship `0c7500c3` · `docs/operations/change-log-2026-07-30.md` §2/§8 · repin PR #375 (`891e50f4`)

## Problem

A phase whose deliverable fails its format contract was re-dispatched to the
**same** CLI forever. A profile's `cli_fallback` chain fires only on infra exit
codes `{80,81,85,124,127}` — never on a contract violation — so a CLI that
systematically mis-formats a deliverable burns every correction, and the
contract-gate breaker then opens, demoting `enforce→advisory` for the rest of
the run (`docs/architecture/contract-block-cli-escalation.md`). The failure
mode is perverse: a FORMAT-compliance failure silently **weakened a gate**. The
correct escape hatch is CLI escalation, not gate demotion.

## Context & evidence

**First occurrence — batch-18, cycles 1171/1172.** agy failed the
adversarial-review contract 7/7 across corrections — corrections re-issued
verbatim converged 0 times in seven, while claude complied — and the circuit
demoted the gate in *both* lanes (`docs/operations/change-log-2026-07-30.md`
§8; the escalation design doc labels the same cycles batch-19). The immediate
console remedy was a repin, not a fix: PR #375 (`891e50f4`, 2026-07-30) moved
the adversarial-review pin to claude/deep "not agy". The class conclusion:
"format adherence turned out to be a **model property past some point, not a
prompt property**" (§8).

**The delay — and why it is the finding.** The fix item
`contract-block-cli-escalation` was filed, claimed into `processing/` — and
stranded there, because the FAIL-side release accounting only ran on a path
wave lanes never take. Change-log §2: stranded "since batch-19 — **which is why
that class recurred twice more while its own fix sat locked away**", invisible
to triage for weeks; nothing retroactively swept old strandings.

**Recurrence — batch-21, cycle-1215.** The same class hit **triage** on the
top-priority lane: deliverable rejected after 2 corrections with
`[failure_context_missing]` — the agent kept emitting a schema-version-1
`evolve-verdict` sentinel where the contract demanded v2 with a structured
failure block (`runs/cycle-1215/audit-fail-reason.json`, phase=triage). Cycle
lost.

**The second stranding.** On 2026-08-04 the deliverable-alignment research
ranked this item #1 of its 7-move portfolio — and found the item **absent from
the live queue entirely**: "the original existed only in the tracked snapshot,
a live instance of the stranding it documents" (research commit `8c5f4285`).
Refiled at 0.96, rank 1 (queue commit `629ba575`).

## Approaches considered

- **Repin the profile away from the weak CLI** (#375): works, shipped, but is a
  manual per-incident remedy — it does not fix the class, and the next weak
  CLI × phase pair starts over.
- **Let the breaker demote**: this *is* the failure mode — the gate's own
  escape hatch weakens the gate.
- **Escalate the phase's primary CLI**: rejected — "the phase ships fine on its
  primary for the 99% PASS path. Escalate only the re-dispatch that already
  failed" (change-log §8). Scoping is a soft overlay on
  `PhaseRequest.ModelRoutingCLI` for that re-dispatch only.
- **Trigger on a locally re-counted correction ordinal**: rejected — it desyncs
  whenever a prior cycle left the breaker hot or the salvage rung consumed a
  block; the trigger uses the contract gate's own `ReviewResult.Blocks`
  counter, the same one that opens the circuit at 3 (design doc, Trigger).
- **Count alone (PR #390's landed shape)**: found insufficient — `Blocks`
  counts blocks, not defects, so two honest, *different* violations read as one
  incapable-CLI signature (research doc §6.1 Gap).
- **Identity as raw string equality**: rejected — the same defect reworded, or
  with different go-test duration tokens, would wrongly suppress escalation.
- **Identity as whole-string compare of the fingerprint-normalized reason**
  (cycle-1289's build): refuted by its own audit — the subset-collapse bug
  below.
- **"Equal ⇒ escalate"**: rejected — the breaker is process-global, so a
  ladder's FIRST block can arrive at `Blocks >= 2` with no prior reason; an
  equality-only rule silently deletes the hot-breaker escape hatch. The rule is
  "prior block known AND differing ⇒ suppress".

## Decision & reasoning

Escalate to a different CLI **family** when two conditions hold: the gate's own
consecutive-block counter reaches 2, and the block is the **same defect** as
the previous one (`contractBlocksShareIdentity`). `Blocks == 0` gates
(evalgate/topngate/triagecap/build floor) never escalate — a different CLI is
not the remedy for a task-binding rejection. `escalationAllowed` runs every
candidate through `policy.ValidatePin`, so escalation can never route to a
family the operator forbade; profiles on disk are untouched (design doc,
Scoping). When the breaker opens anyway, the demotion is still recorded —
stderr WARN, ledger entry, autofile intent through `dispositionrouter` (never a
direct inbox write, which races `inboxmover.Claim`).

## Implementation

**Base mechanism — console, PR #390 (`32671641`, 2026-07-30).** The
console-pipeline sprint landed `contract_escalation.go` (247 lines),
`cyclerun_review.go` wiring, the soft-overlay routing, and the demotion
recorder — rebuilt on main, replacing the orphaned #384. Its review found 5
real defects pre-merge, two severe: escalation was a no-op for *minted* phases
including adversarial-review (the original evidence — the agent-name map
covered only the ten spine phases), and `ChainReviewers` dropped `Demoted` when
a later gate rejected. CI then caught a real transport defect the local machine
could not: the overlay normalized `codex` to a different driver than the
fallback chain's `codex-tmux` on a runner without tmux (change-log §8b).

**Identity hardening — the refiled 0.96 item, loop-landed.**
- *Round 1, cycle-1289 — honest FAIL.* The lane built identity as a
  whole-string compare of the fingerprint-normalized reason; its audit rejected
  it HIGH: *"contractBlocksShareIdentity compares whole summarize() strings, so
  a partially-repaired violation set (subset) reads as a different defect and
  suppresses [escalation]"* (`runs/cycle-1289/audit-fail-reason.json`). Block 1
  reports `{missing_section, missing_verdict}`, the correction closes one,
  block 2 reports `{missing_verdict}` alone — the strongest possible
  incapable-CLI signature, since the CLI demonstrably cannot close the
  remaining violation — yet the strings differ, so escalation was suppressed
  exactly when it was needed.
- *Round 2, cycle-1291 — the subset fix.* The continuation (manifest citing
  cycle-1289's findings) re-primitived identity as the block's
  **violation-code SET**: two blocks are the same defect exactly when their
  code sets intersect — covering subset (partial repair), superset
  (regression), and reordered/reworded renderings, while disjoint sets stay two
  honest defects (design doc's table). Two load-bearing fallbacks: code-less
  reasons fall back to `normalizeReasonForFingerprint` (the breaker's own
  primitive — never a second hashing scheme), and the zero-value identity
  (hot breaker, no prior block) reports true. An import cycle
  (`deliverable` imports `core`) forces the codes to reach `core` as plain data
  via a deliberately narrow regex, so bracketed prose cannot fabricate an
  intersection.
- *Landing.* Cycle-1291 PASSed but its ship hit `GIT_FLEET_REBASE_NEEDED` (a
  peer cycle moved main mid-pipeline; `runs/cycle-1291/ship-error.json`); the
  work landed with cycle-1293's PASS as ship `0c7500c3` (2026-08-04 23:14):
  `contract_escalation.go` (+96), `cyclerun_review.go` wiring, the architecture
  doc, `go/acs/cycle1289` + `cycle1291` predicates, and the §6.1 landing record
  in the research doc.
- *Open tail, cycle-1294.* A follow-on continuation FAILed its audit on a
  deterministic gate: `go vet` found an import cycle in a test importing
  `modelcatalog` from `contract_escalation` — with the auditor narrative
  reading PASS and the vet gate forcing FAIL (verdict-conflict;
  `runs/cycle-1294/audit-fail-reason.json`, dossier
  `knowledge-base/cycles/cycle-1294.md`). Carryover open at time of writing.
  (The caller's framing "subset fix landed cycle-1294" is not what the
  artifacts show: the subset fix is cycle-1291's, landed in `0c7500c3`;
  cycle-1294's dossier records FAIL.)

## Results (measured)

- Three-axis behavior suite against the real `Orchestrator.RunCycle`
  (`contract_escalation_test.go`): NEGATIVE (differing reasons ⇒ no
  escalation — RED before the identity gate), SEMANTIC (same defect,
  duration-token-different text ⇒ still escalates — the discriminator against a
  raw-equality fix), EDGE (hot breaker ⇒ still fires). 10/10
  contract-escalation tests PASS (research doc §6.1).
- The *delay* is itself the measured result: the class recurred twice
  (batch-18 both lanes, batch-21 cycle-1215) while the fix sat in a stranded
  claim (change-log §2) — the deliverable-alignment research's "worst
  fix-latency instance".
- Live escalation-firing counts: no data yet — baseline captured for the
  before/after: contract-gate CIRCUIT OPEN firings = 3 at time of writing, all
  weak-CLI (research doc §6).
- Coda `[session-evidence]`: on 2026-08-05 the breaker demoted on claude-tmux —
  a phase where escalation had no stronger family to reach — the motivating
  evidence for the queued top-family-remedy item.

## Retrospective — what we learned

- **Fix latency is a failure mode of its own.** A correct, designed,
  ranked fix protected nobody for two batches because the queue's *claim
  transport* silently stranded it — twice (processing/ lock, then the live-queue
  absence). The accounting layer needed the same integrity work as the
  deliverables it tracks ([2026-08-batch-integrity-review.md](2026-08-batch-integrity-review.md)).
- **The round-1 FAIL was the system working.** Cycle-1289's audit caught a
  suppression bug precisely in the case the mechanism exists for. Honest FAILs
  on gating mechanisms are cheaper than every alternative.
- **Identity wants the domain's stable vocabulary, not renderings.**
  Violation codes survive rewording, reordering, and partial repair; strings do
  not. Same lesson as the fingerprint-identity generations, independently
  re-derived at this seam.
- **Escalation has a ceiling.** When the failing family is already the
  strongest allowed, the ladder degrades to the old behavior by design — the
  2026-08-05 claude-tmux demotion shows the remaining gap is a remedy for the
  *top* family, not more escalation.

## Links

- Design: `docs/architecture/contract-block-cli-escalation.md` · code
  `go/internal/core/contract_escalation.go`, `go/internal/core/cyclerun_review.go`
- History: `docs/operations/change-log-2026-07-30.md` §2 (stranding), §8/§8b
  (base mechanism) · PR #390 `32671641` · repin #375 `891e50f4` · ship `0c7500c3`
- Runtime: `runs/cycle-1215/audit-fail-reason.json`,
  `runs/cycle-1289/audit-fail-reason.json`, `runs/cycle-1291/ship-error.json`,
  `runs/cycle-1294/audit-fail-reason.json`
- Sibling entries: [2026-08-deliverable-alignment.md](2026-08-deliverable-alignment.md)
  (rank-1 portfolio item; the refile) ·
  [2026-07-llm-output-stability.md](2026-07-llm-output-stability.md)
  (the capability-ceiling evidence behind "model property, not prompt property")
