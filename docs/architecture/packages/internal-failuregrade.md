# internal/failuregrade

## Purpose

`internal/failuregrade` is the graduated-enforcement classifier of [ADR-0048](../adr/0048-work-conservation-fast-reland-resilient-ship.md) Slice A. It grades a gate or cycle failure into a tier. A tiny, provably recoverable defect is then corrected, quarantined or repaired. It no longer costs a whole cycle.

## Design

- **Tiers.** `TierAbort` is the floor and the default: the whole cycle aborts. `TierCorrect` re-dispatches the phase with a correction directive (for example "re-emit the report with challenge token T"). `TierQuarantine` isolates the offending change and warns, without an abort (benign config-file churn). `TierRepair` routes to the typed ship repair ladder (the TOFU `SELF_SHA` re-pin).
- **A closed vocabulary** ([ADR-0047](../adr/0047-surface-classification-and-channel-separation.md), Specification pattern). Only three named classes, each tied to evidence, grade below `TierAbort`.
  - A missing challenge token is always correctable, because the token is known.
  - Tree-diff churn is quarantined only when `ChurnIsBenign`.
  - A `SELF_SHA` mismatch is repaired only when `RebuildVerified`: a reproducible rebuild that came out byte-identical twice.
  - A class whose evidence predicate is unmet stays at `TierAbort`.
- **Safe substring matching.** The signatures (`missing_challenge_token`, `SELF_SHA_TAMPERED`, `tree-diff guard`) are the control reason-codes of this repository, never agent content. The input is the orchestrator's abort reason, not a captured pane, so substring matching is safe here.
- **Fail-closed rendering.** `Tier.String` renders an out-of-range tier as `abort`.

## Invariants

- **The failure floor never silently downgrades** ([ADR-0039](../adr/0039-failure-floor-and-failure-signal-contract.md)). An unknown reason, churn that is not benign, and a `SELF_SHA` mismatch without a verified rebuild (real tampering) all abort. Evidence cannot conjure a grade for an unknown class. Pinned by `TestGrade`.
- **The zero `Evidence` is the safe default** (all false).

## Findings

- Cycles 243, 246 and 247 each lost a whole cycle to a small defect. The defects were a missing challenge token (a 60-second re-emit), benign config-file tree churn, and a `SELF_SHA` mismatch after a verified rebuild.
