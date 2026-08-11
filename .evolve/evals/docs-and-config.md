---
score_cap:
  - criterion: "docs/architecture/phase-recovery.md exists, is git-tracked, and covers timing/failure-diag/phase_retry/backfill+200-char-gate/EVOLVE_BACKFILL_ENABLED"
    max_if_missing: 4
    evidence: "test -f docs/architecture/phase-recovery.md && git ls-files --error-unmatch docs/architecture/phase-recovery.md >/dev/null 2>&1 && grep -qF phase-timing.json docs/architecture/phase-recovery.md && grep -qF EVOLVE_BACKFILL_ENABLED docs/architecture/phase-recovery.md"
  - criterion: "CLAUDE.md env-var table documents EVOLVE_BACKFILL_ENABLED with default 0 (off)"
    max_if_missing: 4
    evidence: "grep -E '\\|.*EVOLVE_BACKFILL_ENABLED.*\\|.*\\b0\\b' CLAUDE.md"
---

# Eval: phase-recovery architecture doc + CLAUDE.md config entry

> Pins the documentation contract for cycle 164's phase-recovery features: a
> canonical `docs/architecture/phase-recovery.md` describing the artifact schemas
> and recovery semantics, plus an `EVOLVE_BACKFILL_ENABLED` row in CLAUDE.md's
> current-behavior env-var table (default `0`, opt-in). The doc evidence uses the
> file-existence DUAL-CHECK (disk + `git ls-files`) because a gitignored or
> un-added doc passes `[ -f ]` in the worktree yet silently vanishes at ship
> (cycle-92 defect mode).
>
> Source incident: cycle 164 — features shipped without canonical docs would be
> undiscoverable; the feedback_doc_stewardship_policy requires everything learned
> to land in docs/.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| arch-doc | phase-recovery.md exists + tracked + topics | 4/10 | `test -f && git ls-files && grep topics` |
| env-row | CLAUDE.md documents EVOLVE_BACKFILL_ENABLED=0 | 4/10 | `grep -E table-row-with-0 CLAUDE.md` |
