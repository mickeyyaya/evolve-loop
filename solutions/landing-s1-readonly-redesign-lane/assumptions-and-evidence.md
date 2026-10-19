# Assumptions and Evidence — landing-s1-readonly-redesign-lane

Every claim in `options/*.md` and `recommendation.md` cites an entry below. `E<n>` entries are sourced
facts. `A<n>` entries are assumptions, each with the probe that would replace it with a measurement.

## Assumptions

- **A1** — A read-only probe (`rev-list`, `diff --cached`, a branch-ahead check) cannot tell a lane that
  main has already superseded from a lane that only needs a rebase, when HEAD equals base. *Basis:* the
  probe sees no commits of the lane, and the staged diff alone does not show whether main holds the same
  content (E2, E3). Probe: run `ClassifyFleetRebaseCandidate` on a staged-only lane fixture.
- **A2** — The scan time that S1 would save is small next to the saving of the content-keyed cache. *Basis:*
  the plan sizes the cache (S5, S6) as the main saving (E1). Probe: the S13 simulation of the plan.
- **A3** — A console item can take the code half at a later date, so recording the decision now loses
  nothing. *Basis:* acceptance criterion 3 allows it (E5). Probe: none needed.

## Evidence

- **E1** — `docs/plans/fleet-landing-at-scale-2026-10.md:315` holds the S1 row. It now says "S1 stays
  dropped" and hands the scan saving to S5 and S6 (`git diff` of this cycle).
- **E2** — `normalizeBuildWorktree` calls `normalizeWorktreeToBase` after the build phase
  (`go/internal/core/phase_bindings.go:277-284`). The lane resets to `WorktreeBaseSHA` with its edits
  staged, so at ship HEAD equals base.
- **E3** — Recovery of `CodeGitFleetRebaseNeeded` first calls `ClassifyFleetRebaseCandidate` and returns
  with no retry on `FleetRebaseAlreadyLanded` (`go/internal/core/ship_recovery.go:39-47`).
- **E4** — Staging a lane and consuming the inbox before `verifyAuditBinding` runs those steps ahead of
  the post-audit tamper guard (inbox item text; scout report of cycle 1960).
- **E5** — The inbox acceptance, items 1–3 (`.evolve/evals/landing-s1-readonly-redesign-lane.md`). Item 3
  accepts a plan that records why S1 stays dropped.
- **E6** — Two review rounds failed the first S1 design (`.evolve/evals/landing-s1-readonly-redesign-lane.md`,
  header note).
