# Retro-fleet stale-worktree: a laundered CRITICAL, recovered
**Period:** 2026-08-03 → 2026-08-04 (cycle-1255 audit through the cycle-1283 landing) · **Status:** shipped (concrete half; named siblings still open)
**Primary artifacts:** `docs/operations/batch-integrity-review-2026-08-04.md` (Finding F1 + its landing record) · `docs/operations/fix-2026-08-04-retro-stale-worktree-fallback.md` · `go/internal/phases/retro/retro.go` · `go/internal/core/cyclerun.go` · landing commit `43a802d3` (cycle-1283) · runtime run dirs `cycle-1278`, `cycle-1280`

## Problem

A fleet lane whose per-cycle worktree had been torn down **lost its
retrospective entirely — a failure in the failure-handler**: the cycle that
most needed a post-mortem was the one that could not produce one (fix doc,
Issue section).

Mechanism, as reproduced by cycle-1255's audit against the compiled binary:
`cs.ActiveWorktree` had exactly one assignment — at worktree creation
(`cyclerun.go:456` at the time) — and nothing cleared it at teardown, so the
persisted cycle state kept naming a pruned directory. `retroWorktree`
(`go/internal/phases/retro/retro.go:88` at the time) passed any **non-empty**
path through verbatim; the bridge's launch guard (`driver_tmux_repl.go`,
`!isDir`) then refused the stale dir with `ExitBadFlags` — **stderr only, no
error return**, silent to the orchestrator (review doc F1; fix doc). The 1255
audit prescribed the fix verbatim: *widen the predicate to
`req.Worktree == "" || !isDir(req.Worktree)`*.

The larger problem is what happened next: the defect was **laundered** — its
identity progressively narrowed across a salvage chain until the unfixed half
vanished from every tracking surface — which is why this entry is as much
about status accounting as about a predicate.

## Context & evidence

The laundering chain (review doc F1, "Gap"; every step's artifact is named
there):

1. Cycle-1255's audit FAILed the fix with the D1 CRITICAL above, reproduced
   against the compiled binary — "the batch's best audit" per the same doc's
   cycle table.
2. The **1268 salvage reframed** the task around the *empty* worktree shape.
3. **Cycle-1270's fault-localization** declared the root cause *"already
   fixed upstream (PR #401)"* — **false**: #401 fixes the never-provisioned
   producer of "no usable worktree", not the torn-down one (see the
   [worktree provisioning entry](2026-08-worktree-provisioning-retry.md)).
   What shipped in `751791ac` covered only the empty shape.
4. The retroactively-shipped `go/acs/cycle1255/predicates_test.go`
   **renamed** the task to `retro-fleet-worktree-EMPTY-fallback`, pinning
   only the fixed subset.
5. Cycle-1272's bookkeeping ship (`68322bdf`) **sealed** the item in
   CHANGELOG as *"already implemented … verified-closed"*, citing a test that
   only covers the empty shape.

"Each cycle's artifacts are individually coherent and locally honest; the
chain as a whole erased a named CRITICAL" (review doc). `""` was never the
shape a torn-down lane produces; all four existing retro worktree tests
passed `worktree=""`, so coverage *looked* complete (fix doc, Gap). The same
vanishing pattern consumed five more named defects (1255-D4, 1267-F2,
1267-F3, 1259-D5/1263-D3), and a structural aggravator made it worse: the
1255 retrospective **did** file the right remediation items — but only inside
`.evolve/runs/cycle-1255/retrospective-report.md`, never reaching the inbox
(a live instance of the lesson-to-action gap; review doc F1).

## Approaches considered

- **The laundered incumbent** — cycle-1270's `req.Worktree == ""` fallback —
  is the refuted approach and gets equal care: it fixed a real shape (empty),
  its tests were green, and it was still the wrong contract, because it
  tested a *string shape* instead of the guard's *predicate*. The narrowing,
  not the code, is what made it look closed (fix doc, Gap).
- **Fallback targets rejected by design** (retro.go doc comment, on main):
  the shared main tree (`req.ProjectRoot`) — refuted by PR #400, since
  `Worktree` is the write-authority predicate; and the dispatching process
  cwd — the exact leak the fleet guard exists to close. With no owned
  workspace there is nowhere safe to mint, so the phase returns `""` and the
  bridge decides exactly as today — never a fabricated path.
- **Re-derive `isDir` locally in the phase** vs export the guard's own
  predicate: rejected — "a launch-refusal predicate with two definitions can
  drift, and this defect is what that drift costs" (fix doc). `isDir` was
  exported as `bridge.IsDir` and the phase consults it.
- **Clear `ActiveWorktree` unconditionally at teardown** vs only on
  successful `Cleanup`: rejected — a *preserved* worktree must keep its path
  or `evolve loop --resume` / `evolve cycle reset` lose the lane, trading a
  stale path for permanently orphaned audited work (the cycle-7 lost-work
  precedent; fix doc).

## Decision & reasoning

Recovery ran in two layers, both queued at the top of the loop queue from the
batch-integrity review (F1, Solution):

- **Concrete** — `retro-fleet-stale-worktree-fallback`, refiled at weight
  **0.96** (the pipeline-integrity band): apply the 1255 prescription
  exactly, with a stale-path regression test (zero stale coverage existed),
  plus the root-cause companion: clear `cs.ActiveWorktree` at lane teardown.
  Contain the symptom *and* remove the source.
- **Class** — `continuation-defect-ledger` (0.95): a continuation/salvage
  lane's audit must diff its deliverable against the ORIGINAL rejecting
  audit's machine-readable `defects[]` and emit per-defect dispositions;
  retro-filed items reach the inbox transactionally; closure claims must cite
  the disposition artifact.

The load-bearing reasoning for the concrete fix: **empty and stale are one
contract — "no usable worktree" — and the only non-drifting way to encode
that is to test the guard's own predicate**, via the same `fleetMode`
key+parser the bridge uses so phase and guard "can never disagree about which
launches fail closed" (retro.go comments on main).

## Implementation

Three rounds, each with its audit on record in the runtime:

- **Cycle-1278 (round 1 — fix right, audit still earned its keep).** The
  audit's narrative verdict was WARN with the core fix verified *correct*:
  RED confirmed by independent `go test -overlay` probes against
  reconstructed pre-fix sources (stale-path and teardown tests genuinely RED
  pre-fix; regression guards green both sides), logic re-derived over all
  four input shapes. But it **named a fresh defect in the fix itself**:
  `clearActiveWorktree` is a *third* read-modify-writer of
  `cycle-state.json` whose read sits outside the sidecar lock, violating the
  ADR-0049 **G7** invariant documented in the write path itself
  (`statejson.go:70-78`) — corroborated by the adversarial review, including
  the self-deadlock trap awaiting the naive fix
  (`cycle-1278/audit-report.md`). The recorded verdict was FAIL via
  verdict-conflict: the deterministic integration-tier gate reported 13
  offenders (the `TestRealTmux_Interactive_*` exit-80 family — a whole-suite
  contention flake class unrelated to this change) and the gate outranks the
  narrative (`cycle-1278/audit-fail-reason.json`).
- **Cycle-1280 (round 2 — the attribution defect).** Rebuild; audit WARN:
  "the code fix is correct, live, and adversarially proven" — both halves
  mutation-tested via `-overlay`, each mutant killing exactly its own
  regression test — but the CHANGELOG amendment, §F1 note, and build report
  attributed the landing to commit `943b1dce`, which is **not in the lane's
  lineage** (it lives on branch `cycle-42824668-1278`): the exact ship-claim
  misattribution class (§F6) this document set exists to close, so it could
  not pass silently (`cycle-1280/audit-report.md`; same integration-tier gate
  forced the recorded FAIL).
- **Cycle-1283 — LANDED** (`43a802d3`, 2026-08-04): `retroWorktree` falls
  back on `fleetMode(req) && !gobridge.IsDir(req.Worktree)`
  (`retro.go:94-99` on main); the teardown callback calls
  `clearActiveWorktree` **only after `o.worktree.Cleanup` succeeds**, as a
  guarded read-modify-write against storage (path guard; preserved worktrees
  keep their path for `--resume`; `cyclerun.go:466-529` on main);
  `bridge.IsDir` exported so phase and guard cannot drift. The CHANGELOG's
  cycle-1272 "verified-closed" line was **struck and corrected in the same
  landing**, with the reason the machine guard missed the over-claim: it
  re-ran the cited proof but never checked that the proof covered the claim
  (F1 landing record). Verified by `go/acs/cycle1283` (5 predicates: 001/003
  discriminate against base `9b129565`, 002/004 pin the anti-over-widening
  axes, 005 asserts the landing record itself), plus `go/acs/cycle1278`,
  `retro_stale_worktree_test.go` and `cyclerun_worktree_teardown_test.go`.

The F1 landing record deliberately separates *prescribed* from *shipped* —
"this doc exists because collapsing that distinction is how the 1255 CRITICAL
got laundered in the first place."

## Results (measured)

- All four retro input shapes are now pinned separately — empty, live, stale,
  and the non-fleet passthrough — RED before the change for the stale and
  teardown cases, PASS after (fix doc, Verification).
- The record was repaired, not just the code: CHANGELOG corrected in the same
  commit, dispositions written into the F1 landing record.
- A small irony in the runtime log: cycle-1283's own provisioning was saved
  by the PR #401 retry (`[worktree] retry 1/2 … cycle-42824668-1283`,
  `loop-console-20260804-205556.log`) — the never-provisioned half of "no
  usable worktree" protecting the landing of the torn-down half's fix.
- **Honestly still open**, per the landing record's own dispositions:
  1255-D4 (symlinked `*_test.go` passes the corpus Walk's suffix filter — the
  `IsRegular` check), 1267-F2 (`DirectImporters` unbounded parse; 512MiB
  allocation probed live), 1267-F3/1270-R-1 (`ScratchCwd` symlink
  hardening) — triage-deferred out of the cycle-1283 fleet scope, **not**
  fixed; plus the 1278 audit's G7 lock follow-up. The class fix
  (`continuation-defect-ledger`) landed separately at cycle-1285 (review
  doc's landing section; see the sibling entry).

## Retrospective — what we learned

- **Integrity lives in the chain, not the cycle.** Every artifact in the
  1255→1268→1270→1272 chain was locally honest; the laundering happened in
  the *handoffs* — a reframe, a false "fixed upstream", a rename, a sealed
  closure. Auditing single cycles cannot catch it; only diffing against the
  original defect list can (hence the ledger class fix).
- **"Already fixed upstream" must be checked against the defect's shape.**
  Never-provisioned and torn-down are different producers of "no usable
  worktree"; citing PR #401 for the latter was plausible and false.
- **Renaming a task is narrowing its identity.** The ACS rename to
  `-EMPTY-fallback` was the laundering's sharpest instrument — the pinned
  name became the pinned scope.
- **Test the guard's predicate, not a string shape** — and export the
  predicate so two definitions cannot drift. The fallback's *target* matters
  as much: never the main tree (write authority, per refuted PR #400), never
  the process cwd, never a fabricated path.
- **Silent refusals launder defects.** `return Exit*, nil` with stderr-only
  reporting is why this survived three salvage cycles; the 1278 audit's
  follow-up list names the sweep.
- **A proof must cover the claim.** The 1272 machine guard re-ran the cited
  test and sealed a closure the test never covered — verification of
  *evidence existence* is not verification of *claim coverage*.
- **The audit rounds were the system working:** round 1 found a new G7
  invariant violation *introduced by the fix*; round 2 refused a
  misattributed ship record. Landing took two extra cycles and was worth
  both.

## Links

- `docs/operations/batch-integrity-review-2026-08-04.md` — F1 forensics + F1
  landing record (the lane's own §3.8 record)
- `docs/operations/fix-2026-08-04-retro-stale-worktree-fallback.md` —
  issue → gap → solution detail
- Runtime evidence: `.evolve/runs/cycle-1278/audit-report.md` (G7 finding),
  `.evolve/runs/cycle-1280/audit-report.md` (attribution finding), both with
  `audit-fail-reason.json` verdict-conflict records
- Sibling entries: [Worktree provisioning retry](2026-08-worktree-provisioning-retry.md)
  — the upstream fix falsely credited by cycle-1270, and the refuted PR #400
  whose write-authority argument shapes `retroWorktree`'s fallback;
  [The continuation defect ledger](2026-08-continuation-defect-ledger.md) —
  the class fix (landed cycle-1285); [The batch integrity
  review](2026-08-batch-integrity-review.md) — the review that surfaced the
  laundering.
