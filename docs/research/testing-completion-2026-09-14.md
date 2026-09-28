# Test campaign completion

Status (2026-09-28): **parked; incomplete.** The campaign stopped without landing anything. Its worktree was salvaged on 2026-09-28. R1 gained structured Go-test evidence through `acsassert.GoTests` (PR #700). R8's always-reporting CI result lands with the port on branch `ci/always-reporting-required-result`. R9 (branch protection) is **not** applied, because it needs the operator's approval. Every other row below is unchanged since the campaign stopped. See [Salvage 2026-09-28](#salvage-2026-09-28).

Starting source: `3a972e20ed55ce7ed9f422843a5ca486354e685f` (2026-09-14). This continuation set out to implement the full remaining scope of the [design](../architecture/test-refactoring-design-2026-09-14.md). The earlier [integration report](../reports/test-refactoring-integration-2026-09-14.md) remains a historical receipt for the first implementation changes.

## Required outcomes

The table is the campaign's record as it stood when the campaign stopped; the State column was not updated by the salvage.

| ID | Requirement | Completion evidence | State |
|---|---|---|---|
| R1 | Host-enforced TDD execution evidence | Real baseline semantic RED and candidate GREEN, complete selected-test inventory, executable-input and environment binding, stale/forged/empty/skipped/compile/infrastructure rejection, enforced production wiring, reviewed merge | In progress |
| R2 | Native simulator repair | Current-installation CLI composition without legacy scripts, truthful phase/ledger/ship outcomes and counts, preservation and failure controls, reviewed merge | In progress |
| R3 | Live failure classification | Structured infrastructure evidence, semantic failures cannot become transient skips, bounded retry and first-attempt accounting, real live execution records, reviewed merge | In progress |
| R4 | Resolve all 47 remaining duplicate candidates | Each old assertion, selector, acceptance identity, fixture and environment mapped to retained execution; baseline/candidate coverage and named fault controls; proven consolidation or a supported distinct-contract retention decision | In progress |
| R5 | Broader fuzzing | Risk-based harness input corpus and executable fuzz targets, bounded real runs, retained minimized failures, scheduled CI, explicit exercised scope | Pending |
| R6 | Measured mutation testing | Passing baseline, reproducible isolated mutants, killed/survived/invalid/timeout denominators, surviving meaningful mutants resolved, no estimated score presented as measured | Pending |
| R7 | Live-provider calibration | Versioned tasks, prompts, model/provider and harness, raw trial outcomes including errors/skips, reviewed expected judgments and held-out cases, real execution and calibration results | Pending |
| R8 | Always-reporting CI result | Unfiltered stable result job on every relevant event; tested selection and expected-skip rules; missing/failed/cancelled required work refuses success; actual GitHub executions | In progress |
| R9 | Branch-rule enforcement | Reviewed required-check configuration applied after the required result exists and passes; read-back proves main is protected without discarding existing protections | In progress |
| R10 | Assertion-by-assertion review of every test | Complete current-source inventory; explicit semantic review of every test, every assertion and delegated oracle; findings resolved or justified; new/changed files re-reviewed; no uncovered or stale review entries | In progress |

The campaign is not complete while any row is pending, only partially evidenced, or stale. A missing provider capability or permission is recorded as a concrete limitation; it does not remove that requirement. No code-size, coverage-percentage or inventory-count target substitutes for semantic review.

## Execution and review protocol

Use independent manual development worktrees from current `origin/main`, with one writer per file. The active runtime checkout is outside this campaign's write scope. Review the contract before implementation; observe a genuine assertion RED for new behavior, implement the minimum, run the same test unchanged to GREEN, and preserve neighboring contracts. Pure consolidation starts from passing tests and requires before/after assertion and execution equivalence, including deliberately broken implementations outside authoritative source trees.

Each implementation receives independent simplification, Go/test correctness and architecture review where relevant. Format, vet, uncached tests and required CI must pass before the normal commit-gate and manual ship path merges a reviewed head. Protect the final combined revision and retain the first failed execution even when a later run passes.

The assertion review covers tracked first-party test entry points in every module, including default, integration, E2E, live, ACS, fuzz, benchmark and example tiers. It also follows test helpers and assertion implementations to their actual failure behavior. Third-party vendored dependencies are enumerated separately if present; they do not become first-party ownership merely by being copied into a checkout.

The review inventory is mechanical evidence of scope, not evidence that any assertion was understood. Each reviewed test records its observable contract, each assertion's role and fault sensitivity, setup/skip behavior, and any delegated oracle. A helper review may be reused only with the helper's source binding and explicit review of each caller's inputs and acceptance meaning. Similar text never authorizes a bulk approval. Missing assertions, weak comparisons, vacuous subprocess selection, ignored child failures, and uncontrolled state are findings to resolve with TDD.

Before completion, regenerate the inventory against the final source and verify complete review coverage, including tests created during this campaign and concurrent main changes. Compare executed selections and coverage independently by OS, toolchain and tier; distinguish tested behavior, intended skips and unavailable capabilities. The final report must link the actual review, execution, PR and external-rule receipts for every requirement above.

## Salvage 2026-09-28

Two campaign worktrees, both based on `3a972e20` and neither committed, were salvaged onto `main`:

| Source worktree | What it held | Where it landed | Requirement |
|---|---|---|---|
| `dev/test-campaign-completion-2026-09-14` | `pkg/acsassert.GoTests` (predicates judge `go test -json` events, not printed PASS text), pilots in `acs/cycle1013`, `acs/cycle1015` and the `cmd/evolve` tokens-report test | PR #700, merged through the wave-29 train (#703, `9db78031`) | R1, partial |
| `dev/test-campaign-completion-2026-09-14` | This tracker | This file (formerly `docs/research/testing-completion-2026-09-14/README.md`) | — |
| `dev/test-ci-required-results-2026-09-14` | `required.yml` with the `CI required` aggregator, `landing-validation.yml`, the trigger moves in `go.yml`, `ci.yml` and `landing-pages.yml`, and the `internal/ciparity` graph, routing and result tests | Branch `ci/always-reporting-required-result`, committed for operator review; the [design](../reports/test-ci-required-results-design-2026-09-14.md) and [implementation](../reports/test-ci-required-results-implementation-2026-09-14.md) reports land with it | R8 |

Not ported: the tracker's `inventory/test-files.tsv` and `inventory/test-inventory.tsv`. They were mechanical snapshots of the `3a972e20` source, and more than 300 commits have landed since. If R10 resumes, it regenerates the inventory from the current source.

Still open:

- R8 needs its first real GitHub execution: the `CI required` check's exact name and publishing app, the reusable-workflow outputs and the selected and skipped callers, observed on a real PR and push.
- R9 is not applied. The design's ruleset payload requires the `CI required` check from GitHub Actions (app `15368`) on `main`. It needs the operator's approval, and before it can be enforced, cycle ships need the native PR publication path the design describes: today they push directly to `main`.
- R2 to R7 and R10 have no salvaged work beyond what their rows record.
