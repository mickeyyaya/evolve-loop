# ADR-0054: Advisor-driven preliminary study cycle

## Status

Accepted on 2026-06-18.

## Context

Large goals need research and decomposition before implementation starts. A single
cycle cannot safely guess dependency order, file ownership, or whether independent
work can run concurrently. The campaign path therefore needs an advisory planning
stage whose output is human-readable, deterministically validated, and executable
without weakening the normal per-cycle integrity floor.

## Decision

Implement the capability in four slices:

- **S1 — wave engine:** `dag.Levels` validates the dependency graph and
  `fleet.PlanWaves` groups file-overlapping tasks into one cycle while allowing
  file-disjoint groups in the same wave to run concurrently.
- **S2 — preliminary study:** the optional `preliminary-study` phase researches
  the goal and emits `campaign-plan.json` plus a narrative study artifact.
- **S3 — CLI driver:** `cmd_campaign.go` exposes `campaign study`, `campaign
  replan`, and `campaign run`. Plans are loaded and verified before rendering or
  execution. Waves execute sequentially through `fleet.Supervisor`; cycles within
  a wave execute concurrently, and a failure triggers one localized retry.
- **S4 — documentation:** this ADR and
  `docs/architecture/campaign-planning-citations.md` record the architecture and
  its research basis.

The preliminary-study model is advisory. `campaign.Plan.Verify`, the DAG planner,
and each launched cycle's existing TDD, audit, and ship gates remain deterministic
trust boundaries. Study and replan display the rendered plan for operator review;
run never executes an invalid plan.

## Consequences

- Multi-cycle goals gain an explicit research and approval boundary.
- Dependency order and file-conflict grouping are reproducible rather than left
  to model judgment at execution time.
- Campaign execution reuses the fleet supervisor and existing cycle isolation,
  so it adds no alternate ship path.
- A failed cycle receives one localized retry; persistent failure stops later
  waves, preserving dependency correctness.
- The study path requires a functioning configured LLM bridge, while
  `campaign run --simulate` remains deterministic for plumbing verification.
