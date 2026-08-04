# The continuation defect ledger: five rounds to an honest mechanism
**Period:** 2026-08-04 (cycles 1279–1292, one day) · **Status:** shipped
**Primary artifacts:** `docs/architecture/continuation-defect-ledger.md` · `docs/operations/batch-integrity-review-2026-08-04.md` (F1 + landing records) · ships `37631b5e` (cycle-1287), `3d5932a7` (cycle-1290), `278627ec` (cycle-1292) · runtime `/…/evolve-loop-runtime/.evolve/runs/cycle-{1279,1281,1282,1285}/audit-fail-reason.json`

## Problem

The batch integrity review's F1 finding
([2026-08-batch-integrity-review.md](2026-08-batch-integrity-review.md)): a
CRITICAL defect raised by cycle-1255's audit survived the
`1255 → 1268-salvage → 1270 → 1272` chain and ended up recorded as "verified
closed" without anyone fixing it. No step lied; each continuation narrowed the
wording, renamed the task, or declared it handled upstream, and the defect was
erased **collectively**. The retrospective half had the same shape: cycle-1255's
retro "filed" remediation items that existed only inside its own report — the
queue never saw them.

The design doc (`docs/architecture/continuation-defect-ledger.md`) names the
common gap: **the machine held no addressable record that outlived the cycle
that raised it.** A rejecting audit's defects lived as prose; prose cannot be
diffed by a gate. And `faillearn.WriteArtifacts` guaranteed the retrospective
survives an outage but nothing about the work it asks for.

## Context & evidence

Queued from the review at weight 0.95 (`continuation-defect-ledger`, F1/F2/F6
class solution, review §4). The mandate had three clauses: (i) a continuation's
audit must diff its deliverable against the ORIGINAL rejecting audit's
machine-readable defect list and emit per-defect dispositions — an audit that
cannot account for a named defect cannot PASS; (ii) retro-filed remediation
items are written to `.evolve/inbox` transactionally with the retrospective;
(iii) closure claims must cite the disposition artifact.

What makes this entry worth a chronicle is not the design — it is that the
mechanism took **five FAIL rounds in one day** to survive its own audits, and
every round's defect was real. Each round is pinned by the runtime
`audit-fail-reason.json` of its cycle.

## Approaches considered

The design rejected, with reasons recorded in the doc:

- **Prose reconciliation** (status quo): refuted by the 1255 chain itself.
- **Trusting workspace state as disposition truth**: refuted at round 3 by a
  working proof-of-concept (below) — agent-writable state cannot ground a gate.
- **Caps that erase on overflow**: rejected — "a cap that erases defects is
  laundering wearing a resource-limit costume"; overflow mints an OPEN marker
  row a continuation must inherit.
- **Negation markers on closure claims** ("not closed" exempts the line):
  rejected as a bypass cheaper than the citation demanded.
- **Whole-document citation** for closure claims: rejected — one incidental
  mention would vouch for twenty unevidenced claims; the gate is line-scoped.
- **Rollback of already-written inbox items on partial failure**: deliberately
  not attempted — a real remediation item left queued is the safe direction.
- **Blocking on a missing ancestor ledger**: rejected — ancestors predating the
  mechanism are legitimate; WARN naming the ancestor, never silence (D2).

## Decision & reasoning

Both halves hang off `hooks.Classify` — the audit verdict seam — so nothing is
reachable only from a helper. A FAIL/WARN with structured defects persists
`defect-ledger.json` (rows with stable content-derived ids, text stored
**verbatim** — narrowing the text is the laundering). A cycle carrying a
`continuation-manifest.json` must account for every inherited OPEN row via
`defect-dispositions.json`; unaccounted rows block the PASS by id. Entries
transition, never delete: **"a ledger that shrinks is a ledger that launders"**
is the invariant every later fix preserved. The degrade posture is deliberately
asymmetric (doc table): missing dispositions FAIL (that *is* the defect);
failing to write the emit-side record only WARNs (the record of a rejection
must not change the rejection).

## Implementation

The five rounds, each keyed to its runtime artifact:

**Round 1 — cycle-1279 (build; audit FAIL).** The mechanism was built and its
own audit rejected it: *"CRITICAL: reconcileContinuationDefects rebuilds the
ledger from ancestor.Entries only and truncate-writes it (defect_ledger.go:
199-224), erasing entries emitDef[ectLedger appended]"*
(`runs/cycle-1279/audit-fail-reason.json`). An ordinary second `Classify` made
the anti-laundering ledger shrink itself — no adversary required. Ids were
positional, so any shrink re-bound an id to different text. Fix (recorded as D1
of the doc's D1–D7 hardening table): merge onto the workspace ledger indexed by
id; ids become `defectID(text)` content hashes.

**Round 2 — cycle-1281 (continuation; audit FAIL).** *"writeIfAbsent treats any
existing path at `<id>.json` as success while retrofile.ExistingItemIDs
deliberately skips unparseable JSON, so a truncated/non-item file [suppresses
the item]"* (`runs/cycle-1281/audit-fail-reason.json`). The transactional inbox
guarantee could report success while dropping the remediation — the exact 1255
state, reachable through a silent *skip* rather than a write *error*. The
class was closed across this and the 1285 landing (DEF-4 record in the review
doc): `writeIfAbsent` reports skips; a skip is tolerated only when the on-disk
file is byte-identical, and fails loudly by id otherwise.

**Round 3 — cycle-1282 (continuation; audit FAIL).** *"CRITICAL:
defect_ledger.go:307-309 accepts the workspace's own agent-writable
defect-ledger.json as disposition truth — a pre-planted ledger returns PASS
with [zero disposition evidence]"* (`runs/cycle-1282/audit-fail-reason.json`;
PoC `TestPOC_A`, `advpoc/` in the run dir). The graded agent could pre-plant a
ledger marking inherited ids FIXED, and the ID-keyed merge substituted the
planted text under the real id — the tamper-evident record holding the
tamperer's text. The audit's full list (DEF-1..6, review doc's cycle-1285
landing) added: "evidence" meant any file that exists (`/etc/hosts` closed a
CRITICAL — `TestPOC_B`/`C`); 32-bit ids with a last-row-wins index
(`TestPOC_D`); unbounded agent-authored growth. Fixes: inherited rows rebuilt
from the ANCESTOR every pass, status derived only from
`defect-dispositions.json`, trusted-row arm deleted; evidence must be
relative, non-escaping, project-root-resolved, and never the mechanism's own
bookkeeping; ids widened to 16 bytes, FIRST row wins, text drift is a blocking
diagnostic; 64-entry/2000-rune caps that RECORD overflow.

**Round 4 — cycle-1285 (continuation; audit FAIL).** *"HIGH:
faillearn/writer.go:30-32 writes the inbox before the retrospective and aborts
on its failure, so two agent-authored defect lines sharing a 60-rune
remedi[ation-slug prefix collide]"* (`runs/cycle-1285/audit-fail-reason.json`).
The inbox-first ordering — itself the correct fix for "filed in the report,
absent from the queue" — interacted with a lossy slug (60-rune cap plus
punctuation collapse used as identity): two distinct defects minted one id, the
collision refusal aborted `WriteArtifacts` before the retrospective, and the
caller downgraded that to a stderr WARN. No retrospective, no lesson, one item
for two defects. Fix (1287 continuation table, F1): `remediationFingerprint`
appends a digest of the FULL title so ids are injective; the ordering was
deliberately NOT reversed — the collision was the cause, not the ordering. Four
sibling defects from the same audit (arming via a deletable 0644 manifest,
case-insensitive self-vouching compare, quoted-span false positives,
stat-then-write race) were closed alongside, with the reproducer made
tree-resident (`repro_cycle1285_test.go` in both `phases/audit` and `core`).

**Round 5 — PASS, with one last lesson about ships.** The lane's work went
fully green in the cycle-1286 run — every phase PASS/WARN — and then the ship
itself FAILed three times on a push-strand
(`knowledge-base/cycles/cycle-1286.md`; P0 consumed with the root cause fixed
pre-halt, queue commit `629ba575`). It landed as cycle-1287 PASS, ship
`37631b5e`: `defect_ledger.go` (563 lines), `closure_claim.go` (the clause-iii
line-scoped citation gate), the transactional `faillearn` inbox, adversarial
test suites, and predicates for cycles 1279/1282/1285/1287. Note honestly: the
deliverable-alignment research doc records this as "landed cycle-1286" — a
lane-label attribution of the kind F6 warns about; the dossiers and the ship
commit say the green landing is cycle-1287's.

**Post-landing continuations (the crucible kept working).** Cycle-1287's audit
raised two more: the floor published artifacts at 0600 while everything else
publishes 0644 (dropped silently by the 1285 publish-path rewrite because no
test pinned the mode), and a disk-level inbox failure suppressed the failure
*analysis* along with the queue write. Both closed in cycle-1290 (ship
`3d5932a7`): chmod the temp before the link-publish; `retrospective-unqueued.md`
published as a distinct, explicitly degraded artifact — reusing the canonical
name would have required weakening the 1255 invariant lock. Cycle-1290's audit
then caught the mechanism's own docs making an unbacked deferral claim (the
cited queue item didn't exist on disk — the audit role guard couldn't write it)
and the degraded artifact overclaiming ("every item UNQUEUED" after a partial
write). Both closed in cycle-1292 (ship `278627ec`): the entry filed by the
build phase and verified through the real consumer `inboxbatch.LoadDir`
(`TestC1292_004` — a stat would green on an item the loader drops), and
`unqueuedItems` reading the inbox BACK to name only what is genuinely absent.

## Results (measured)

- Five FAIL rounds (1279, 1281, 1282, 1285, plus the 1286 ship-strand), four
  distinct real mechanism defects, then PASS — the research doc's summary: "5
  rounds, 4 distinct real defects, then a PASS … find-one-solve-one operating
  as a hardening crucible" (`docs/research/deliverable-alignment-2026-08/README.md` §1).
- Every closed defect has a named regression lock in the tree (e.g.
  `TestAdversarial_PrePlantedWorkspaceLedgerCannotDisposition`,
  `TestWriteInboxItems_CollidingFilenameIsNotSilentlyDropped`,
  `TestC1290_001`, `TestC1292_004`) and a `defect-dispositions.json` record —
  the mechanism's own artifact, applied to itself.
- The closure-claim gate grades the project's own documents: the design doc's
  clause-iii disposition names `defect-dispositions.json` on the claiming line
  because the gate it describes requires it.
- Known ceilings recorded, not papered over (doc): a workspace with both the
  manifest and `lane-scope.json` destroyed is unrecoverable from the registry;
  the degraded diagnosis needs a `runDir`; the inbox read-back is
  point-in-time.

## Retrospective — what we learned

- **A mechanism that will GATE other work must survive security-grade
  adversarial probing.** Every round's defect was a way the anti-laundering
  ledger could itself launder — truncate-erase, silent skip, planted trust
  root, id collision. A gate's threat model includes the graded party writing
  the gate's inputs; round 3's PoC proved agent-writable state can never be
  disposition truth.
- **Find-one-solve-one is a hardening crucible, not an architecture.** Each
  found defect was real and narrower than the last — exactly the mode the
  deliverable-alignment research says the verification/accounting layers should
  run in ([2026-08-deliverable-alignment.md](2026-08-deliverable-alignment.md)).
- **Fix the cause, keep the invariant.** Twice the cheap fix was to reverse a
  correct ordering or weaken a correct lock; both times the landing preserved
  the invariant and fixed the actual defect (collision, not ordering;
  distinct degraded artifact, not a weakened canonical one). One reproducer was
  caught green-by-skip and strengthened — the pattern the parent review files
  as a finding.
- **The record of the record needs the same rules.** The lineage's own docs
  produced an unbacked deferral claim and a lane-label landing attribution —
  found and closed by the same machinery, which is the strongest evidence the
  mechanism was needed.

## Links

- Design + full defect tables: `docs/architecture/continuation-defect-ledger.md`
- Landing records: `docs/operations/batch-integrity-review-2026-08-04.md`
  (cycle-1285/1287/1290/1292 sections)
- Runtime evidence: `evolve-loop-runtime/.evolve/runs/cycle-{1279,1281,1282,1285}/audit-fail-reason.json`, `cycle-1282/advpoc/`
- Sibling entries: [2026-08-batch-integrity-review.md](2026-08-batch-integrity-review.md) (the F1 finding this closes) ·
  [2026-08-deliverable-alignment.md](2026-08-deliverable-alignment.md) (rank-7 accounting layer; the crucible framing) ·
  [2026-08-retro-fleet-stale-worktree.md](2026-08-retro-fleet-stale-worktree.md) (the laundered defect itself)
