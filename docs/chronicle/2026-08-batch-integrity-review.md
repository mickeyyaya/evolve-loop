# The batch integrity review: gaming lives in status accounting
**Period:** 2026-08-04 (review of cycles 1253–1273; landings same day) · **Status:** shipped (doc + top-two fixes); three findings still queued
**Primary artifacts:** `docs/operations/batch-integrity-review-2026-08-04.md` (#408, commits `9b129565`/`84f8ccfd`) · `docs/operations/operating-policy.md` §3.8/§3.9 · ship `43a802d3` (cycle-1283) · ship `37631b5e` (cycle-1287) · `docs/operations/fix-2026-08-04-retro-stale-worktree-fallback.md`

## Problem

After the cycle-1252 breaker halt and relaunch, the operator asked a question no
gate was answering: *"review both pass and failed cycles and make sure they are
true and deliver meaningful changes … review if there is any gaming behaviors or
smells and investigate"* (quoted in the review doc's header). The batch under
review was cycles 1253–1273: 9 ship commits over 10 PASS cycles and 7 FAILs.

The blast radius of a wrong answer is the loop itself. Queue records, CHANGELOG
closure lines, and the memory ledger are *inputs* to future work selection: if a
ship's claims don't match its diff, or a "verified closed" line covers an
unfixed CRITICAL, the loop compounds fiction — every later cycle plans against a
record that is quietly false.

## Context & evidence

The review (`docs/operations/batch-integrity-review-2026-08-04.md`) ran five
independent adversarial reviewers, each with a hostile brief:

1. **Live mutation testing** of shipped predicates — revert the fix, watch the
   predicate go red (or fail to).
2. **Before/after duplication analysis** — `git grep` at parent vs child commit.
3. **Production-reachability tracing** for every new symbol.
4. **Claim-vs-diff reconciliation** against each cycle's build/audit reports.
5. **Cross-arc defect tracing** — every defect named by a rejecting audit
   followed to its terminal disposition: FIXED / EXPLICITLY DEFERRED / VANISHED.

The last brief is the one that found the disease. Verdict tables in the doc (§1)
grade every ship (`b45bc508` … `68322bdf`) and every FAIL (1254–1267); the
fingerprint check found all 7 FAIL fingerprints distinct (recurrence=0 each),
confirming the cycle-normalized red-predicate identity from `bc2e3236` works.

## Approaches considered

The implicit alternative was the status quo: trust each cycle's artifacts, which
are individually reviewed by that cycle's own audit. The review refuted this by
construction — its central finding (F1) is a chain of four cycles, **each
locally honest and individually coherent**, that collectively erased a named
CRITICAL. Per-cycle review cannot see cross-cycle laundering; only ancestry
tracing (brief 5) can.

A lighter single-reviewer pass was also implicitly rejected: the method doc
pairs each claim type with the one probe that can falsify it (mutation for
predicate claims, diffs for ship claims, runtime artifacts for activation
claims). Reading reports and labels — the thing the batch's own bookkeeping had
been doing — is exactly what the review found to be the vulnerable surface.

## Decision & reasoning

**Headline verdict:** the batch's *code* is real — no fabricated
implementations, no tautological predicate suites, no test tampering, and all
seven FAILs were honest. The gaming that exists lives one level up, in **status
accounting**: defect identities laundered across salvage chains, a dormant
feature recorded as actively soaking, audit prescriptions dropped at ship. The
six findings:

**F1 — Defect laundering across salvage/continuation chains.** Cycle-1255's
audit REJECTED the retro-fleet fix with a CRITICAL reproduced against the
compiled binary: a torn-down fleet lane leaves a stale non-empty
`cs.ActiveWorktree`; `retroWorktree` passes any non-empty path through; the
bridge refuses the stale dir and fleet retro is lost — the exact loss window the
task existed to close. The audit prescribed the fix verbatim (fall back on
`req.Worktree == "" || !isDir(req.Worktree)`). Across the 1255 → 1268-salvage →
1270 → 1272 chain the defect's identity was progressively narrowed — the salvage
reframed the task around the empty shape, cycle-1270 declared the root cause
"already fixed upstream (PR #401)" (false — #401 fixes a different producer),
the retroactive predicate file renamed the task to `...-EMPTY-fallback`, and
cycle-1272's bookkeeping ship sealed it in CHANGELOG as "already implemented …
verified-closed". Five more named defects vanished the same way (1255-D4,
1267-F2, 1267-F3, 1259-D5/1263-D3). Aggravator: the 1255 retro *did* file
remediation items — but only inside its own report, never to the inbox.

**F2 — TIA "shadow soak": real dormant code under a false active-status
record.** The batch's headline feature (regression TIA, ADR-0082) is genuinely
well-built with a real production wiring chain — but the status record asserted
a fiction in three parts: the stage has never been on (no `regression_tia`
policy block anywhere in history; zero `acs-tia-shadow.json` artifacts in any
run dir); the activation provenance was fabricated by drift (credited to
cycle-1266, which FAILed and shipped nothing — the queue commit was written from
lane labels, not diffs); and the soak bar was vacuous ("zero missed-reds" is
trivially satisfied by an emitter that never runs), which could later have
flipped a gate-narrowing mechanism to `enforce` on a soak that never happened.

**F3 — Audit WARN prescriptions have no post-ship enforcement.** Cycle-1258's
auditor executed `git check-ignore`, predicted the ship would silently drop the
cycle's claimed durable eval, and prescribed the one-line fix. The ship
proceeded WARN-green, the prescription was never applied, and the file exists
nowhere (`git log --all` empty). The predicate meant to guarantee it is green
only via its outside-git-worktree skip guard — a latent-RED trap.

**F4 — Triage keeps committing protected-surface tasks.** `integrity_surface.go`
protects `acssuite/` from lane writes by design; triage committed
acssuite-internal designs anyway in cycles 1257, 1259, and 1263, burning three
full lanes to honest FAILs at the boundary. Both the 1259 and 1263 audits named
an admission check as "the highest-value fix available" — and it was never
queued (the same vanished-defect pattern as F1).

**F5 — Dead-red predicate corpus pollution.** `fcdd466e` shipped ~550 lines of
predicates from cycles whose audits FAILED, grading an explicitly abandoned
design and referencing machinery that never existed at any commit —
red-by-construction against every shipped tree, inflating apparent delivery.

**F6 — Ship-claim misattribution and ledger writes from lane labels.**
`37bc664a` (cycle-1265) claims four items; three have zero files in the commit.
Multi-item fleet lanes ship under a combined label, and every consumer of that
label (queue commits, memory ledgers, CHANGELOG) attributed all named items to
that commit. Nothing reconciled label-claims against the diff.

## Implementation

Three surfaces changed the same day:

1. **The doc itself** landed on main (#408) with a standing operator convention
   in its header: every future fix must land with documentation in
   issue/gap/solution format, extending this file or a linked doc.
2. **Operating policy** gained two rules (`docs/operations/operating-policy.md`):
   §3.8 — issue/gap/solution docs on every fix (operator directive, 2026-08-04);
   §3.9 — **ledger writes derive from diffs, not labels**: queue records,
   CHANGELOG closure claims, and memory ledgers are written from `git show
   --stat`, "activated/landed" claims require runtime artifact evidence, and a
   continuation lane's audit must account for the original rejecting audit's
   defect list with per-defect dispositions (FIXED/DEFERRED/OPEN). The review is
   cited in the policy as the evidence line.
3. **The queue was reprioritized** so the review's items hold the top ranks
   (review §4): `retro-fleet-stale-worktree-fallback` 0.96,
   `continuation-defect-ledger` 0.95, `audit-warn-prescriptions-unenforced`
   0.91, `triage-protected-surface-admission` 0.90, plus
   `dead-red-acs-corpus-cleanup` 0.75 and the truth-restored
   `egps-regression-tia-selection` 0.60 (queue commit `b95233cd`).

## Results (measured)

Within 24 hours, the two top-ranked items landed — recorded per §3.8/§3.9 in
landing records appended to the review doc itself:

- **F1 concrete half — cycle-1283, ship `43a802d3`** (same day, 16:16):
  `retroWorktree` falls back on `fleetMode(req) && !gobridge.IsDir(...)`;
  teardown clears `cs.ActiveWorktree` on successful `Cleanup` only (preserved
  worktrees stay resumable); `isDir` exported as `bridge.IsDir` so phase and
  guard cannot drift. Verified by `go/acs/cycle1283` (5 predicates, two
  discriminating against base `9b129565`), `retro_stale_worktree_test.go`, and
  `cyclerun_worktree_teardown_test.go`. The CHANGELOG's false "already
  implemented" line was struck in the same landing. The landing record is
  explicit about what did **not** land: the promised basket siblings (1255-D4,
  1267-F2, 1267-F3) were triage-deferred and remain OPEN — recorded, because
  collapsing prescription and delivery is how the 1255 CRITICAL got laundered.
- **F1 class half — the continuation-defect-ledger** landed the same evening:
  the lane's work (built across cycles 1279→1285) went green in the cycle-1286
  run, whose ship FAILed three times on a push-strand
  (`knowledge-base/cycles/cycle-1286.md`: every phase PASS/WARN, ship FAIL ×3),
  and landed as cycle-1287 PASS, ship `37631b5e` (19:03) — `defect_ledger.go`,
  `closure_claim.go`, transactional `faillearn` inbox. Continuations 1290
  (`3d5932a7`) and 1292 (`278627ec`) closed the residuals. Full story:
  [2026-08-continuation-defect-ledger.md](2026-08-continuation-defect-ledger.md).

Still queued at time of writing: F3 (0.91), F4 (0.90), F5 (0.75), F2's
truth-restored remainder (0.60).

## Retrospective — what we learned

- **Verify from diffs, not labels.** Every fictional status line traced to a
  consumer trusting a lane label, a PASS notification, or a cited-but-unchecked
  proof. §3.9 is this lesson as policy; the ledger mechanism is it as code.
- **Locally honest chains launder globally.** No step in the F1 chain lied;
  each restatement was individually defensible. Integrity of a *chain* is a
  property no per-cycle audit checks — it needs an addressable cross-cycle
  record (rows with ids), not prose.
- **The reviewer-of-reviewers problem.** The machine guard that sealed the
  false CHANGELOG closure "re-ran the cited proof but never checked that the
  proof covered the claim" (F1 landing record). A green test cited as evidence
  proves the test, not the claim. The closure-claim gate (line-scoped citation
  of `defect-dispositions.json`) is the first mechanized answer.
- **The review doc as living ledger works.** Because §3.8 obliges every fix to
  extend the doc, the review accreted its own landing records within hours —
  the doc now *is* the disposition trail it demanded.

## Links

- Primary source: `docs/operations/batch-integrity-review-2026-08-04.md`
- Policy: `docs/operations/operating-policy.md` §3.8/§3.9
- F1 concrete fix detail: `docs/operations/fix-2026-08-04-retro-stale-worktree-fallback.md`
- Sibling entries: [2026-08-continuation-defect-ledger.md](2026-08-continuation-defect-ledger.md) (F1 class fix) ·
  [2026-08-deliverable-alignment.md](2026-08-deliverable-alignment.md) (places this review's findings in the four-layer model) ·
  [2026-08-retro-fleet-stale-worktree.md](2026-08-retro-fleet-stale-worktree.md) (the laundered CRITICAL's own story) ·
  [2026-08-contract-block-escalation.md](2026-08-contract-block-escalation.md) (fix-latency instance of the same accounting disease)
