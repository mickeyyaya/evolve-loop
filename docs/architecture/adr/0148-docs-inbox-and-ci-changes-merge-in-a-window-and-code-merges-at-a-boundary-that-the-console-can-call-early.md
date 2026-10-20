# ADR-0148: docs, inbox and CI changes merge in a window; code merges at a boundary that the console can call early

- **Status:** Accepted (2026-10-20). The operator chose "windows + on-demand". The window stays off until `fleet.integration.stage` is `enforce` (ADR-0147).
- **Plan:** [pr-merge-latency-2026-10](../../plans/pr-merge-latency-2026-10.md). **Research:** [wave-latency-2026-10](../../research/wave-latency-2026-10.md) §6, [latency-gates-2026-10](../../research/latency-gates-2026-10.md). **Design:** [internal-mergewindow.md](../packages/internal-mergewindow.md).
- **Relates to:** [ADR-0144](0144-latency-is-gated-by-invariants-at-merge-and-by-trends-on-main.md), which gates latency fixes at merge. ADR-0147 (`docs/architecture/adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md`, from the branch `docs/wave-integration`) is the prerequisite: at `fleet.integration.stage: enforce` the wave orchestrator owns integration and no lane writes `main`. ADR-0147 is Proposed until its PR merges.
- **Evidence:**
  - Green PRs waited 61 PR-hours for a boundary over #979 to #1005. The median wait was 1.2 h, and the maximum was 9.6 h (plan F2).
  - The median time from open to the first green run was 0.4 h (plan F1).
  - A merge in a wave strands the next lane push (`go/internal/phases/ship/landing/push.go:191-196`, plan F5).
  - `cmd/evolve` depends on 270 of 302 internal packages, so only a docs, inbox or CI change has no Go dependent (O5).

## Context

Each PR merges only at a wave boundary. Most PRs are green long before the boundary, so the wait is the largest part of their life. A code merge in a wave changes the base of the lanes and strands the next lane push. A docs, inbox or CI change has no Go dependent and does not change the plane binary.

## Decision

1. Drop the `.github/` paths of a PR. When CI routes the rest to no Go and no landing suite, it is a window PR.
2. A window PR merges when green, also in a wave.
3. Each other PR is a boundary PR. It merges only at a boundary.
4. The CI router of `required.yml` defines the path classes. `evolve pr window` reads that table, so one rule serves both.
5. The policy key `pr.merge_window.stage` is `off` by default. At `off`, each PR is a boundary PR.
6. Below `fleet.integration.stage: enforce` (ADR-0147, plan D15) each PR is a boundary PR, at each value of `pr.merge_window.stage`.
7. The console calls an early boundary when a green boundary PR waits and the pool runs below its window.

## Consequences

- Good: a docs or inbox PR no longer waits up to 9.6 h for a boundary.
- Good: CI routing and the merge window cannot drift, because one table defines both.
- Bad: a change to the CI router also changes the merge window. A reviewer of `required.yml` must know this.
- Bad: `CLAUDE.md` and `AGENTS.md` are docs to CI, so they are window paths (plan Q1).
- Bad: a docs PR that also adds inbox items is a boundary PR, because CI runs the Go suite for it.
- Bad: an early boundary stops lanes before their wave ends. The pool must be below its window, so the loss is small.

## Rejected

- **GitHub merge queue:** it merges code in a wave, which strands lanes. It also runs CI again on each group on `merge_group`.
- **On-demand boundary only:** each docs PR still waits for a boundary.
- **Keep as is:** the 61 PR-hours of wait stay.
