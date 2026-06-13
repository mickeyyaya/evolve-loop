# ADR-0048: Work Conservation — Graduated Enforcement, Content-Addressed Audit Reuse, Resilient Ship

- **Status:** Proposed (design-first; build in approved slices, like ADR-0046 L3)
- **Date:** 2026-06-13
- **Driver:** the cycles 243–248 batch retrospective (P1/P2/P5,
  `knowledge-base/research/batch-retrospective-cycles-243-248-2026-06-07.md`) — three
  inbox proposals (`graduated-enforcement`, `content-addressed-audit-reuse`,
  `ship-transactionality`) that share one theme: **the loop destroys or re-does work it
  has already proven safe.** A 60-second-fixable report defect aborts a whole cycle; a
  bit-identical re-land re-runs a 50-minute pipeline; a ship that dies between commit and
  push strands `main`.
- **Relates to:** ADR-0034/0035 (deliverable contracts), ADR-0039 (failure floor + correction
  ladder), ADR-0044/0045 (phase recovery / corrective interaction), the ship repair ladder
  (v16.8.0), CA.* run-identity/OCC work.

## Problem

Three independent waste/fragility sources, one principle violated — **conserve proven work**:

1. **Whole-cycle abort on a tiny, correctable defect** (P1). Three cycle-killers each cost a
   full cycle when a targeted correction would have cost seconds: (a) a missing/incorrect
   `challenge-token` in a report at audit time; (b) benign config-file tree-churn tripping the
   tree-diff guard; (c) a `SELF_SHA` mismatch after a legitimate rebuild. The correction-retry
   tier (PR #60, contract-correction) already exists for deliverable-contract violations — it
   just isn't extended to these.
2. **Re-auditing bit-identical content** (P2). Cycles 247–248 re-ran ~50-minute pipelines to
   re-land content cycle 246 had already audited PASS; the operator proved content-parity by
   hand (diffstat match). The step-5 provenance headers already record
   `{cycle, phase, tree_sha, inputs_digest}` per audited verdict — nothing consults them on a
   re-land.
3. **Non-atomic ship** (P5). Cycle 246's ship committed to local `main`, then died at
   `verify-self-sha` *before push*, stranding `main` ahead-by-1 — a state every later observer
   misread and an operator had to untangle.

## Decision

Three slices, each an instance of *conserve proven work*, ordered by blast-radius (lowest first):

### Slice A — Graduated enforcement (extend the correction ladder)

A gate failure is not binary (PASS / abort). Classify each into a **tier**, reusing the existing
ADR-0039 correction ladder (Strategy pattern over a `FailureClass → Tier` map):

| Killer | Today | Tier |
|---|---|---|
| missing/incorrect challenge-token in report | audit FAIL → abort | **correct**: re-dispatch the phase with a "re-emit the report with token `<T>`" directive (the ADR-0045 correction-directive mechanism), bounded by the existing retry cap |
| benign-class config tree-churn at tree-diff guard | whole-cycle abort | **quarantine+warn**: content matching a revert or a known-benign signature is quarantined and warned, not aborted (the ship-repair "collider quarantine" pattern, generalized) |
| `SELF_SHA` mismatch after a verified rebuild | abort | **repair**: the TOFU re-pin already exists in the ship repair ladder — route this class there, never to abort |

Design: a `gradeFailure(class, evidence) → {Abort|Correct|Quarantine|Repair}` pure classifier
(unit-testable per class), consulted before the abort path. **Floor invariant preserved**: only
the three named, evidence-anchored classes are graded; everything else still aborts (closed
vocabulary — ADR-0047 Specification pattern). Mutation test per class.

### Slice B — Content-addressed audit reuse (fast re-land)

Bind an audit PASS verdict to the **content** it audited, not the cycle that produced it. The
step-5 provenance header already carries `tree_sha` + `inputs_digest`. Add a `verdictcache`
keyed by `(tree_sha, inputs_digest)`:

- On ship/pre-pipeline, compute the worktree's `tree_sha` (`git rev-parse HEAD^{tree}` /
  `write-tree`). If it **exactly equals** a recorded PASS verdict's `tree_sha` AND the
  `inputs_digest` matches, skip tdd/build/audit and carry the prior verdict forward (the
  audit-binding already binds to the change; this binds the *decision* to the change).
- Acceptance: a re-land cycle whose worktree tree-SHA equals an audited verdict's `tree_sha`
  goes straight to ship with the prior verdict's provenance; a one-byte change misses the cache
  and re-runs fully.

Design: content-addressed lookup (a Value Object key, never a cycle number). **Safety**: exact
SHA match only — no fuzzy/diffstat matching (the operator's manual diffstat proof becomes a
deterministic equality). The cache is advisory and self-invalidating: a miss costs a full run,
so a lost/corrupt cache only costs time, never correctness (same degradation contract as
`clihealth`).

> **Implementation note (2026-06-13, shadow stage shipped):** the assumed `inputs_digest`
> **does not exist as a machine-computed value** — it is only a parse-only field in
> agent-emitted `phasecoherence` provenance Markdown; no Go code computes or persists it, and
> it is absent from the ledger and every sidecar. The canonical content identity that *does*
> exist and is already ledger-persisted is `LedgerEntry.WorktreeTreeSHA` = `git write-tree` of
> the staged worktree changes (the same value ship verifies in the Slice C1 pre-commit binding).
> A git tree object hash is itself a content-addressed, collision-resistant identity of the
> code. **The shadow stage therefore keys on `worktree_tree_sha` alone** (`internal/verdictcache`,
> populated as a projection of the audit binding in `recordAuditBinding`; observe-only probe in
> `RunCycle` pre-loop, mirroring the Slice A shadow precedent — no dial). `inputs_digest` (to
> additionally pin eval-set / goal identity) is a **deferred refinement for the enforce stage**,
> where actually *skipping* phases on a match warrants the stronger key; enforce also adds the
> `EVOLVE_VERDICT_CACHE` (`off`/`shadow`/`enforce`, default `shadow`) dial per ADR-0046 discipline.

### Slice C — Resilient ship (all-or-nothing)

Make the commit→verify→push sequence transactional. Two options; **recommend C1**:

- **C1 — verify-before-mutate (reorder):** move every verification (self-SHA, attestation,
  ff-check) BEFORE the first mutation (`git commit`). Once verification passes, commit+push is
  the only remaining work; a death there leaves an unpushed commit that the existing push-only
  resume (ship repair ladder) already completes. Smallest change, removes the
  "commit-then-fail-verify" window entirely.
- **C2 — ship-in-progress marker + resume:** write a `ship-inflight.json` before commit;
  the next loop invocation honors it (complete-the-push or rollback-the-local-commit). More
  moving parts; only needed if some verification genuinely cannot precede commit.

Acceptance: kill ship between commit and push → next invocation either completes the push or
rolls back the local commit; `main` is never left ahead-of-remote silently.

## Build order & gating

Lowest blast-radius first: **A → C1 → B**. Each slice: TDD (per-class / per-acceptance) +
dual-review + `/commit` + binary-rebuild, shipped independently. Each is shadow-able where it
changes a gate decision (grade in shadow logs would-correct; verdict-cache in shadow logs
would-skip) before enforce, matching the ADR-0046 soak→enforce discipline.

## Non-goals / open items (separate dispositions)

- **`ledger-line-1740-historical-damage`** is NOT covered here — it is an **operator decision**,
  not a code change. The damage is genuine (a predecessor's bytes were rewritten post-hoc,
  cycle-107 era) and `evolve ledger verify` correctly stays RED on it (the benign seam classes
  are already accepted with tight signatures, L3.3). The operator must choose: (1) **epoch-anchor**
  — declare a known-good genesis at a post-damage line and verify forward from it (preserves the
  evidence, stops the false-RED on the live chain); or (2) **rebaseline** — rewrite from an anchor
  (destructive, loses the damaged segment). Recommend **(1) epoch-anchor**: non-destructive,
  keeps the historical record, and is the minimal change to make `verify` green on go-forward
  while the damaged segment stays auditable. Needs an `evolve ledger anchor <seq>` command +
  operator sign-off.
- **`loop-binary-self-deploy`** (ADR-0046 L3) stays design-only pending an approved slice plan.
- **`bridge-ratelimit` remainder** is ADR-0047 Stage 4 (per-CLI footer/PaneRegion scoping).
- **`cli-deliberate-refresh-and-canary`** component (4) needs live-CLI execution (canary drives a
  real generation per CLI); build with a driver seam so the decision logic is unit-testable and
  only the live drive is integration-gated.
