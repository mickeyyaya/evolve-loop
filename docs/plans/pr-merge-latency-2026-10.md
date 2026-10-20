# Plan: a green docs, inbox or CI PR merges in a window, and a code PR merges at a boundary that the console can call early

- **Status:** accepted, 2026-10-20. P2 is built in stage `off`. P2b and P3 to P5 are open. P1 is shelved: ADR-0147 replaces it.
- **Decision:** [ADR-0148](../architecture/adr/0148-docs-inbox-and-ci-changes-merge-in-a-window-and-code-merges-at-a-boundary-that-the-console-can-call-early.md). **Research:** [wave-latency-2026-10](../research/wave-latency-2026-10.md) §6 (O1 to O5), [latency-gates-2026-10](../research/latency-gates-2026-10.md). **Design:** [internal-mergewindow.md](../architecture/packages/internal-mergewindow.md).
- **Operator guide:** [wave-operations.md](../operations/wave-operations.md) §5, [operating-policy.md](../operations/operating-policy.md) §2 item 5.

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [5. Decisions](#5-decisions)
- [6. Components](#6-components)
- [7. Order](#7-order)
- [8. Verification](#8-verification)
- [9. Risks](#9-risks)
- [10. Open questions](#10-open-questions)

## Terms used throughout

- **Boundary:** the time between two waves, when no run lease is live. Today each PR merges only at a boundary.
- **Window:** a time inside a wave when a PR merges while lanes run.
- **Window PR:** a PR whose changed paths are all docs, inbox items or CI files, by the rule in §5 D2.
- **Boundary PR:** each other PR. A code PR is a boundary PR.
- **Early boundary:** a boundary that the console calls with the loop brake (`evolve loop-stop --wait`) before the wave ends by itself.
- **Origin move:** a commit on `origin/main` that a lane did not start from.

## 1. Request

The operator decided on 2026-10-20: "windows + on-demand". Docs-only, inbox-only and CI-only PRs merge as soon as they are green, also in a wave. Code PRs stay boundary-only. The console calls an early boundary when green code PRs wait and the pool runs below its width.

The operator first set one condition: merge windows wait until P1 lands. P1 was a console PR for a lane ship that recovers from an external origin move. P1 is now shelved. Decision D15 of the wave-integration plan sets a new condition: merge windows wait for `fleet.integration.stage: enforce`. At that stage the wave orchestrator owns integration, and no lane writes `main`. The decision is ADR-0147, `docs/architecture/adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md` on the branch `docs/wave-integration`.

## 2. Facts

The console measured 27 PRs (#979 to #1005) on 2026-10-20 with `gh`. The data has, for each PR: open to first green, green to merge, CI runs, heads, failed runs and CI minutes.

| Id | Fact | Source |
|---|---|---|
| F1 | The median time from open to the first green run is 0.4 h. | measurement, 2026-10-20 |
| F2 | Green PRs waited 61 PR-hours for a boundary. The median wait is 1.2 h, and the maximum is 9.6 h (#995). | measurement, 2026-10-20 |
| F3 | Four PRs (#983, #984, #986, #997) used 33 of their 43 PR-hours to reach green. Their reds came from causes outside their change. The causes were a stale STE lint base, a race on `main`, the coverage gate on old code and the tree-reader lists. | measurement, 2026-10-20 |
| F4 | Rebase churn on shared files gave 2 to 6 CI runs for each PR and about 2000 CI-minutes. The files are `docs/research-index.md` (each docs PR appends to it), `go/.cover-strict`, the generated `docs/architecture/signal-codes.md`, and stacked squash merges. | measurement, 2026-10-20 |
| F5 | A merge in a wave strands the next lane push: the lane ship does not recover from an origin move (`go/internal/phases/ship/landing/push.go:191-196`, `divergedOrigin`). | code read, 2026-10-20 |
| F6 | `evolve pr merge` refuses each PR while any run lease is live (`go/cmd/evolve/cmd_pr.go`, `planeIdle`, called from `mergeAll` and `mergeOne`). | code read, 2026-10-20 |
| F7 | The CI router `changes` in `.github/workflows/required.yml` (#988) already sorts each path into classes. Docs paths run no Go or landing suite, and inbox paths run only the inbox lint. | `.github/workflows/required.yml`, job `changes` |
| F8 | A merge queue tests changes together. It uses overlap to decide which changes it tests together, not to block a change (O1, O3). | research O1, O3 |
| F9 | A gate on the composed tree is necessary. Before its merge queue, 7% to 10% of the CI failures of Jira came from the merge step (O4). | research O4 |
| F10 | In this module `cmd/evolve` depends on 270 of the 302 internal packages, so two Go edits nearly always share a dependent (O5). Only a docs, inbox or CI change has no Go dependent. | research O5 |
| F11 | The GitHub merge queue needs each required workflow to run on the `merge_group` event. It tests each group again on a temporary branch ([humanwhocodes](https://humanwhocodes.com/blog/2026/04/improving-developer-velocity-github-merge-queue), 2026-04; [qaskills](https://qaskills.sh/blog/merge-queue-testing-required-checks), acc. 2026-10-20). | web, 2026-10-20 |
| F12 | A latency fix is gated by an invariant at merge and by a trend on `main` (ADR-0144). | research G1 to G12 |

*Implication (synthesis):* the merge wait (F2) is larger than the CI time (F1). A rule that lets the PRs with no Go dependent (F10) merge in a wave removes most of the wait. The rule must wait for the ADR-0147 integration stage (F5).

## 3. Goals and non-goals

Goals:

- G1: a green window PR merges in less than 0.5 h at the median, also in a wave.
- G2: a green boundary PR waits less than 2 h at the maximum, because the console calls an early boundary.
- G3: a PR goes red only for a cause in its own change (P4).
- G4: a PR needs at most 2 CI runs at the median (P3).

Non-goals:

- A code PR does not merge in a wave. The plane binary, the pinned ship SHA and the lane bases stay one state in a wave.
- This plan does not change the CI suites. It reuses the CI router.

## 4. Candidate designs

| Criterion (weight) | A. Windows and on-demand boundary | B. GitHub merge queue | C. On-demand boundary only | D. Keep as is |
|---|---|---|---|---|
| Cuts the window-PR wait (3) | 3: merge when green | 2: merge after a group run | 1: waits for the next early boundary | 0 |
| Cuts the code-PR wait (2) | 2: early boundary | 1: it merges code in a wave, which no lane-side fix makes safe | 2 | 0 |
| Keeps the wave state whole (3) | 3: code merges only at a boundary | 0: it merges code at any time (F5) | 3 | 3 |
| CI cost (2) | 2: no new runs | 0: one more run for each group on `merge_group` (F11) | 2 | 1: rebase churn stays (F4) |
| Build cost (1) | 2: one verb, one policy key, the ADR-0147 stage | 1: workflow changes on each required job | 3 | 3 |
| **Weighted total** | **28** | **9** | **23** | **14** |

A wins because it removes the largest wait (F2) and keeps code merges at a boundary. B does not know the wave: it merges a code PR while lanes run. It also adds a group CI run to a workflow with a median wall time of 22.4 min (latency-gates §2). With C, each docs PR still waits for a boundary. D keeps the 61 PR-hours of F2.

**What changes our mind:** the ADR-0147 integration and the measures can show that a code merge in a wave strands no lane over 3 waves. Then a merge queue for code PRs (B) is a candidate again.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Choose design A. | F2, F10, §4 | B: F5, F11. C: window PRs keep their wait. D: F2. |
| D2 | A path under `.github/` is a window path. A path that the CI router routes to no Go suite and no landing suite is a window path. Each other path is a boundary path. | F7, F10 | A second list of path classes in Go: it drifts from CI. |
| D3 | The rule reads its classes from the case tables of `required.yml` at run time, and it fails closed on a table that it cannot read. | F7 | A Go copy with a parity test: two copies of one rule. A shared data file: it changes the required CI job in a live wave. |
| D4 | The policy key `pr.merge_window.stage` has the values `off` and `on`. The compiled default is `off`. At `off` each PR is a boundary PR, with the reason "merge windows wait for fleet.integration.stage enforce (ADR-0147)". | F5 | Turn the window on at once: F5 strands lanes. |
| D5 | The operator sets the stage to `on` only after `fleet.integration.stage` is `enforce` (ADR-0147, wave-integration D15). | F5 | Wait for P1, a fix to the lane ship: P1 is shelved, and wave-integration F2.7 shows that a lane-side fix cannot be complete. |
| D6 | The console calls an early boundary when a green boundary PR waits longer than a policy bound and the pool runs below its window. | F2 | A fixed boundary clock: it stops waves that have work. |
| D7 | Remove the rebase hotspots and keep `main` green (P3, P4). | F3, F4 | |

### Worked example

PR #1005 changed only files under `docs/` (`gh pr diff 1005 --name-only`, 2026-10-20). The verb gives:

```
$ evolve pr window --files-from files.txt
boundary: merge windows wait for fleet.integration.stage enforce (ADR-0147) (stage off, paths window)
```

`paths window` shows that the rule finds only window paths. At stage `on`, the same PR exits 0 with `window: every path is docs, inbox or CI-only`. A PR that also changes `go/.cover-strict` exits 3 and names that path.

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| P1 | Shelved. ADR-0147 replaces it: at `fleet.integration.stage: enforce` no lane writes `main`, so a window merge strands no lane push. | the ADR-0147 plan owns the tests | wave-integration plan |
| P2 | `evolve pr window <n> \| --files-from FILE [--json]` and `pr.merge_window.stage` (this PR, stage `off`). | `TestClassify_TheLiveCIRoutingTableDecidesEachPath`, `TestPRWindow_UntilIntegrationEnforceEveryPRIsABoundaryAndTheRuleStillShows` | console, built |
| P2b | `evolve pr merge` takes a window PR while a loop is live, and the console calls an early boundary (D6). | a live loop and stage `on`: a docs PR merges, a `go/` PR is refused | inbox `pr-merge-latency-p2b-window-consumers`, console |
| P3 | One file for each research-index entry and a generated index with a check. A verb regenerates the generated files on a rebase. Stacked PRs are discouraged. | two docs PRs merge in each order with no conflict | inbox `pr-merge-latency-p3-conflict-hotspots`, console |
| P4 | Fix or retire the stale ACS predicates of cycles 43, 813 and 1801. `evolve ci classify` files a fix-first item for a base red. | the predicates pass on `main` under `-tags acs` | inbox `pr-merge-latency-p4-keep-main-green`, console |
| P5 | A loop write to a curated inbox field never blocks `sync-main`. | a replay of the cycle 2000 write | inbox `pr-merge-latency-p5-boundary-self-heal`, console |
| — | Operator decision: the lane STE check moves from `shadow` to `enforce` (cycle 1979 shipped a doc that failed the strict lint). | a failed lane doc goes back to the build | inbox `lane-ste-lint-stage-enforce`, operator |

### How P2 works

1. The verb reads `pr.merge_window.stage` from `.evolve/policy.json`.
2. It reads `.github/workflows/required.yml` and parses the two case tables of the job `changes` (`mergewindow.Parse`).
3. It gets the changed paths. For a PR it calls `gh api --paginate repos/{owner}/{repo}/pulls/<n>/files` and takes each file name and each old name of a rename. A test gives a file list with `--files-from`.
4. It classifies each path (`Rule.Classify`). One boundary path makes the PR a boundary PR.
5. At stage `off` the verdict is `boundary`. The JSON field `rule` keeps the class of the paths. Thus the operator can count the window PRs.

| Exit | Meaning |
|---|---|
| 0 | `window`: the PR can merge in a wave. |
| 3 | `boundary`: the PR waits for a boundary, or the stage is `off`. |
| 2 | A read or a parse failed: the policy, `required.yml`, the file list or `gh`. |
| 10 | A usage error. |

## 7. Order

1. P2 at stage `off` (this PR). It changes no behavior, and it lets the console measure the window PRs.
2. ADR-0147 at `fleet.integration.stage: enforce`. It removes the cause of F5.
3. P2b, at stage `off`. It uses the verb in `pr merge` and adds the early boundary. At `off` it changes no merge.
4. The operator sets `pr.merge_window.stage` to `on` (D5). Before P2b, no verb can merge a window PR in a wave, so this step comes last.
5. P4, then P3, then P5. P4 removes the largest cause of red (F3). P3 removes the churn (F4). P5 removes a boundary block.

## 8. Verification

| Goal | Before (2026-10-20) | Measure after |
|---|---|---|
| G1 | window PRs waited with all others: median 1.2 h | the time from green to merge of window PRs over 20 PRs |
| G2 | maximum 9.6 h | the maximum time from green to merge of boundary PRs over 20 PRs |
| G3 | 33 of 43 PR-hours on reds outside the change (4 PRs) | the count of PRs whose first red is also red on the base |
| G4 | 2 to 6 CI runs for each PR | the median of CI runs for each PR |

P2 is verified by tests at 100% coverage of the new code. The tests read the live `required.yml`, so a change of the CI router changes the classes that the tests check. Use the same `gh` query as F2 to measure after P2b.

## 9. Risks

| Risk | Control |
|---|---|
| A window merge strands a lane before the ADR-0147 stage is `enforce`. | The stage default is `off` (D4). |
| A change to the CI router changes the window rule. | That is the intent (D3). The parser fails closed on an unknown flag, a missing catch-all arm or an unknown glob. |
| `CLAUDE.md` and `AGENTS.md` are docs to CI, so they are window paths. A lane that starts after the merge reads the new instructions. | The same is true at a boundary. See open question Q1. |
| A rename hides a code path. | The verb reads the old name of each rename. |
| A path under `.github/` is a window path, also `required.yml`, the table that the verb reads. | The operator asked for CI-only PRs in the window. A CI file does not change the plane binary or a lane base. The PR still needs green CI and a review. |
| A file name with a space or a newline splits into window paths. | The verb reads one JSON string for each name from `gh`, and one name for each line from `--files-from`. |

## 10. Open questions

- Q1: must a change to `CLAUDE.md` or `AGENTS.md` wait for a boundary? If yes, the CI router must route it as non-docs, so the two rules stay one rule.
- Q2: what is the bound for the early boundary in D6? The plan proposes 1 h, the median wait of F2. P2b sets it as a policy value.
