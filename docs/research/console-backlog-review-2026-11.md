# Research: the console backlog — which items to drop, merge, narrow or send to lanes

- **Status:** findings and proposed decisions, 2026-11-19. The operator approves each decision before the console changes the inbox.
- **Related:** [console-build-or-route-2026-11](console-build-or-route-2026-11.md) (when the console builds an item), [admission-cmd-evolve-cell-2026-11](../plans/admission-cmd-evolve-cell-2026-11.md) (the `cmd/evolve` cell), [incident 2026-11-14: shipped work stayed open in the inbox](../incidents/2026-11-14-shipped-work-stayed-open-in-the-inbox.md).

## Table of contents

1. [Request](#1-request)
2. [Terms used throughout](#2-terms-used-throughout)
3. [Method](#3-method)
4. [Findings](#4-findings)
5. [Candidate approaches](#5-candidate-approaches)
6. [Proposed decisions](#6-proposed-decisions)
7. [Pipeline fixes that stop the pile-up](#7-pipeline-fixes-that-stop-the-pile-up)
8. [Limits](#8-limits)
9. [Appendix: item lists](#9-appendix-item-lists)

## 1. Request

The operator asked on 2026-11-19: "Could we have the design and architecture review to learn which group tasks can be merged or dropped".

The context: wave 181 ran with 2 of 8 lanes busy. `evolve wave supply` showed 1 admissible item of 319. The console owned 280 items, and the console builds one item at a time. So the console backlog sets the lane width, and the backlog must shrink.

## 2. Terms used throughout

| Term | Meaning |
|---|---|
| Console item | An inbox item that only the console can build: it touches a protected path, has a console-only kind, or carries a console route. |
| Drop | Consume the item with a resolution that cites evidence on `origin/main`. |
| Merge | Make one item from several items that change the same files for one goal. |
| Narrow | Keep the item, but cut its acceptance to the criteria that are still open. |
| Lane | Route the item back to the lanes, because no protected path is necessary. |
| Salvage branch | A `cycle-cd3ae73e-<N>` branch that holds one "salvage snapshot (ADR-0076 continuation-on-fail)" commit of a failed cycle. |

## 3. Method

The review had two passes. A first pass gave each item one disposition. A second, adversarial pass tried to refute each claim that removes an item from the console queue.

| Step | Who | Input | Output |
|---|---|---|---|
| 1 | console, deterministic | `evolve inbox list --status console --json` | 280 items in 4 groups |
| 2 | console, deterministic | the 49 `gc-flag-unmerged-*` items | branch state from `git rev-list`, `git cherry`, `git diff`, and the ledger `inbox-lifecycle` entries |
| 3 | 3 seats, `evolve agent run architecture-design` (agy) | groups of 109, 87 and 35 items | one disposition per item, with evidence on `origin/main` (`b32f7063f`) |
| 4 | console, deterministic | the cited commits, symbols and paths | 4 of 4 commits, 12 of 12 symbols and 8 of 8 paths exist on `origin/main` |
| 5 | 2 seats, `evolve agent run adversarial-review` (claude) | 40 claims: each DONE, OBSOLETE and DUPLICATE claim, and each LANE claim on an item held as protected | CONFIRMED or REFUTED against each acceptance criterion |

All seats were read-only. No inbox state changed during the review.

## 4. Findings

### 4.1 The shape of the backlog

**F1.1** The console owns 280 items (`evolve inbox list --status console`, 2026-11-19). By the reason the intake gives:

| Reason | Items |
|---|---:|
| `route:console-manual` | 144 |
| `kind:<k>` (a console-only kind) | 64 |
| `protected fix surface: <path>` | 38 |
| `route:console` | 32 |
| `route:console-sandbox-denied` | 1 |
| `mentions protected path` | 1 |

*Implication:* 177 items (63%) are console-owned only because a person or a stage set a route. The route is not a fact about the code.

### 4.2 The gc flag items

**F2.1** 49 items are `gc-flag-unmerged-cycle-cd3ae73e-<N>-<hash>`. They name 33 branches. 16 branches have two items.

**F2.2** The cause of the second item is the flag key. `FlagKey` joins the action, the branch and the worktree path (`go/internal/gc/flagtally.go:35`). The first gc runs see "branch not merged into HEAD" with a worktree path. After the worktree goes, gc sees "unmerged orphan branch (no worktree)" with an empty path. The key changes, so the tally starts again and a second item is minted (`flagtally.go:72-86`).

**F2.3** All 33 branches hold exactly one commit, "salvage snapshot (ADR-0076 continuation-on-fail)". `git cherry origin/main` marks each commit as not on main.

**F2.4** The ledger `inbox-lifecycle` entries name the item that each salvage cycle claimed:

| State of the claimed item now | Branches |
|---|---:|
| Consumed or processed by a later cycle | 18 |
| No inbox item | 1 |
| In processing in a live cycle (2530) | 1 |
| Still open on the console (9 distinct items) | 13 |

*Implication:* 18 salvage branches hold work that a later cycle replaced. Their 26 flag items carry no work. The 15 other branches (23 flag items) can seed the build of their open item. The branch with no inbox item (`cycle-cd3ae73e-2365`) is in this group, because no record proves that a later cycle replaced its work.

### 4.3 The first pass is not proof

**F3.1** The first pass marked 31 items DONE. The adversarial pass confirmed 15 and refuted 16.

**F3.2** A refuted item had real code on main, but one acceptance criterion was open. Examples:

- `cmd-evolve-cell-is-too-coarse-for-8-lanes`: `admission.FileCells` exists (`go/internal/admission/admission.go:54`) and the loop passes it on (`go/cmd/evolve/cmd_loop_orchestrator.go:108`). But `.evolve/policy.json` on main sets no `fleet.orchestrator.file_cells`, and the plan is "Status: proposed". The lane pool on 2026-11-19 had 18 items in `waiting_cell`.
- `ledger-restructure-c12-incremental-verify`: `VerifyScope` (`go/internal/adapters/ledger/ledger.go:233`) seeds from `sealedPredecessor` (`go/internal/adapters/ledger/seal.go:281-297`). That function reads only the last line of the newest sealed segment. The adversarial seat concluded that a change in an older segment passes the incremental verify.
- `model-tier-policy-p2-catalog-tier-pins`: the acceptance says a pin to an absent model makes the refresh fail. The code keeps the live pick and warns (`go/internal/modelquery/query.go:85-96`), and a test pins that behavior.

**F3.3** The first pass marked 5 items OBSOLETE. The adversarial pass confirmed 2. It refuted 3 items that are blocked on unmerged work, not replaced: `gate-child-env-allowlist` and `main-red-stop-provable-means-a-lane-can-prove-it` wait on branches, and `edit-layout-c5-gate-lists-as-fragments` is deferred in its plan.

**F3.4** The first pass marked 1 item DUPLICATE: `rebuild-pr-1254-acs-compile` asks for the change of `acs-packages-compile-and-ci-guards-them`. The adversarial pass confirmed it. In total, 37 first-pass claims remove an item, and 18 of them hold.

*Implication:* a symbol on main does not prove an acceptance criterion. A drop needs a check of each criterion. With the first pass alone, the console drops 19 items with open work.

### 4.4 Merge clusters

**F4.1** The first pass found 6 clusters with one goal and shared files:

| Cluster | Members | Result |
|---|---|---|
| blocker proof | `blocker-proof-accepts-a-protected-acs-predicate`, `blocker-proof-cannot-explain-a-guard-failure-of-a-protected-file` | 1 console item. The third member, `blocker-proof-refuses-a-protected-test-file`, closes with [PR #1570](https://github.com/mickeyyaya/evolve-loop/pull/1570) (open, CI green, commit `85ef08f`). |
| wave 158 latency breach | `latency-wave-breach-from-wave-158-ac-1-console`, `-ac-2-console` | 1 console item. Both name `go/internal/acssuite/golane.go`; ac-1 is the re-check of ac-2. |
| read-isolation diagnostics | `contract-row-n16-fence-outruns-the-open-record`, `sandbox-reports-stopped-on-the-host` | 1 console item on `go/internal/bridge/read_isolation.go` |
| boot refresh | `boot-refresh-rearms-the-wave-handoff`, `boundary-refresh-builds-a-stranded-head` | 1 lane item on `go/internal/loopchain/` |
| final audit criteria | `audit-evaluation-contract-e4-final-audit-criteria`, `code-review-phase-q5-audit-on-the-index` | 1 console item: both edit the auditor persona |
| delta review | `audit-evaluation-contract-e5-ledger-criterion-review-plan`, `code-review-phase-q4-delta-re-review` | 1 console item: both edit `codereview` and the defect ledger |

**F4.2** The first pass checked 13 other clusters and kept them apart, for example the `test-audit-t8a/b/c` shards and the `agent-messaging` M5 to M7 chain. Each has disjoint files or an explicit dependency order.

### 4.5 Items that can go to lanes

**F5.1** The first pass marked 35 items LANE. The adversarial pass checked the 3 items that the intake held as protected and confirmed all 3. Two of them need an edit of `files[]` first: `config-single-authority-sweep` and `docs-drift-sweep-2026-07` still list a protected path whose part of the work is on main.

**F5.2** The adversarial pass did not check the other 32 LANE claims. A wrong LANE claim costs one lane claim: the lane triage refuses a protected file and the item bounces back. A wrong drop loses work. So the check went to the drops first.

### 4.6 Items that need an operator decision

**F6.1** 16 items need a decision, not a build:

- 9 `failure-class-*` items: `retrofile` stubs with no files, no summary and no acceptance, at a recurrence of 1 or 2.
- 3 items held as public good-first issues: `inbox-list-names-no-root`, `read-verbs-create-evolve-in-the-cwd`, `wave-status-cost-line-uses-a-second-count`.
- `pipeline-repair-cycle-2354`: a stub with no files and no defect text.
- `inbox-prioritization-p5-reason-on-priority-change`: the change removes the recurrence `Reweight` seam. The frozen predicates `TestC1932_005` to `TestC1932_008` pin that seam (`go/acs/cycle1932/predicates_test.go:242-300`).
- `cli-ship-force-with-lease-lane`: the change conflicts with `go/internal/phases/ship/landing/push_test.go:214`, which bans `--force-with-lease` in the landing.
- `fable-simulation-slices-f1-f5`: part of it shipped as `skills/fable/COMPACT.md`.

### 4.7 Why the backlog grows

The seats named the same pipeline causes from three groups:

| Id | Cause | Evidence |
|---|---|---|
| F7.1 | Work that lands outside a cycle ship does not consume its item. | 15 confirmed DONE items; incident 2026-11-14 |
| F7.2 | A route reason stays after the protected manifest is narrowed from a directory to single files. | `agy-tier-map-single-source`, `bridge-tests-build-evolve-once` and others in F5.1 |
| F7.3 | Every `kind:pipeline-*` item goes to the console, even when no file is protected. | `ci-macos-fixed-parallelism`, `core-tests-cut-git-starts` |
| F7.4 | `retrofile` files stubs with no files and no acceptance. | the 9 `failure-class-*` items |
| F7.5 | A split leaves its parent open, and gc mints a second item per branch. | `latency-wave-breach-from-wave-158`; F2.2 |
| F7.6 | Text-based protection scans free text when `files[]` is empty. | `practice-claude-code-changelog-1bd0f3ce-202611140548` |

### 4.8 Outside practice

**F8.1** Backlog guides agree on three steps: merge duplicates into one item, remove obsolete items, and prune on a fixed cadence. Their age limits range from 90 days to 18 months, and no standard exists ([Mountain Goat Software](https://www.mountaingoatsoftware.com/agile/product-backlog/product-backlog-health), [GitScrum](https://docs.gitscrum.com/en/how-to/backlog-maintenance-best-practices), [Alex Debecker](https://alexdebecker.substack.com/p/how-i-refine-my-backlog)).

**F8.2** A close by age has a cost. A study of 20 projects found faster pull request reviews after the Stale bot came in. The same projects then had fewer active contributors ([Khatoonabadi et al. 2023](https://www.mozillafoundation.org/en/research/library/understanding-the-helpfulness-of-stale-bot-for-pull-based-development)).

**F8.3** Current duplicate detection finds candidates by similarity, then a final judgment step decides ([GitHub changelog 2026-06-18](https://github.blog/changelog/2026-06-18-duplicate-detection-and-issue-fields-mcp-support-for-github-issues), [Linear](https://linear.app/blog/using-ai-to-detect-similar-issues), [arXiv 2504.14797](https://arxiv.org/html/2504.14797v1)).

*Implication (synthesis):* this review follows the same shape: candidates first, then a judgment step. F3.1 shows that the judgment step is necessary here.

## 5. Candidate approaches

| Criterion (weight) | A. Per-item disposition with an adversarial check | B. Send every `route:console*` item to lanes | C. Drop items by age | D. No change |
|---|---|---|---|---|
| Keeps open work (0.4) | 5: each drop has a check of each criterion | 3: no drop, but protected items bounce | 1: F3.1 shows that open work hides in old items | 5 |
| Lane supply (0.3) | 4: 35 lane candidates | 5: 177 items | 2 | 1: 1 admissible item |
| Console queue (0.3) | 4: about 118 fewer items | 4: 177 fewer at first, but each protected item bounces back | 4: many fewer, by age alone | 1 |
| **Score** | **4.40** | 3.90 | 2.20 | 2.60 |

Approach A wins because it is the only approach that removes items and keeps open work. Approach B moves the check from the console to the lane triage and pays one wasted claim for each wrong item. Approach C is the Stale-bot pattern of F8.2.

## 6. Proposed decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Consume the 18 confirmed items (15 DONE, 2 OBSOLETE, 1 DUPLICATE), each with its evidence as the resolution. | F3.1, F3.3 | Drop all 37 first-pass claims: 19 have open work (F3.1). |
| D2 | Narrow the 19 refuted items: cut each acceptance to its open criteria, and park the 3 blocked items with `--blocked-by` their branch or plan. | F3.2, F3.3 | Keep the full acceptance: the console then rebuilds work that is on main. |
| D3 | Consume the 26 gc flag items of the 18 replaced salvage branches, then delete those 18 branches. | F2.4 | Keep them: gc never removes a flagged item, so they stay forever. |
| D4 | Consume the 23 gc flag items of the other 15 branches. Add each branch name to its open item as a seed. Keep `cycle-cd3ae73e-2365` until a person reads its commit. | F2.1, F2.4 | Delete those branches: they hold the last try at open work. |
| D5 | Make 6 items from the 12 cluster members (one member closes with PR #1570). | F4.1 | Merge the 13 kept-apart clusters: F4.2. |
| D6 | Route the 35 LANE items to lanes one at a time, with `evolve inbox route-lane`. Edit `files[]` first on the 2 items of F5.1. | F5.1, F5.2 | Route them in one batch: a bounce then hides which claim was wrong. |
| D7 | Operator: retire the 9 `failure-class-*` stubs and `pipeline-repair-cycle-2354`, and decide the 6 other items of F6.1. | F6.1 | Build the stubs: they hold no defect to build. |
| D8 | Operator: set `fleet.orchestrator.file_cells` for `cmd/evolve`, after the plan [admission-cmd-evolve-cell-2026-11](../plans/admission-cmd-evolve-cell-2026-11.md) passes its review. | F3.2 | Leave it off: 18 lane items wait on one cell. |

**Effect:** the console queue goes from 280 to about 160 items (synthesis: 280 − 18 − 49 − 6 − 35 − 10 = 162, before the 6 other DECIDE outcomes). The lane supply grows by up to 35 items, and by the `waiting_cell` items after D8.

**Order:** D1 to D7 change inbox state, and a mid-wave edit, route or withdraw is refused. So they run at the next boundary, one verb at a time, with the full output of each verb read. D3 deletes branches only after `evolve gc` confirms that no worktree and no live cycle uses them.

**What changes our mind:** a D6 item that bounces back from a lane is evidence that the LANE check of F5.2 is too weak. Then the 32 unchecked LANE claims get the adversarial pass before they move.

## 7. Pipeline fixes that stop the pile-up

Each fix becomes one inbox item. A fix that edits no protected path routes to lanes.

| Cause | Fix |
|---|---|
| F2.2 | Key a branch flag on the action and the branch only, so that a lost worktree does not mint a second item. |
| F7.1 | A sweep that consumes an item when its acceptance holds on main after a landing outside a cycle (the `closedBy` sweep of the 2026-11-14 incident). |
| F7.2 | `inbox curate` recomputes the route reason against the current manifest and reports each stale `protected fix surface` reason. |
| F7.3 | `kind:pipeline-*` routes to the console only when a file in `files[]` is protected. |
| F7.4 | `retrofile` refuses a stub with no files and no acceptance, or folds a recurrence into the existing self-heal incident. |
| F7.5 | A split consumes its parent in the same step. |
| F7.6 | Text-based protection does not run on a research or practice item; such an item must declare `files[]`. |

## 8. Limits

- The 32 unchecked LANE claims (F5.2) and the 130 CONSOLE claims had one pass only. A CONSOLE claim that is wrong keeps an item off lanes; it loses no work.
- The adversarial seats used the same `origin/main` (`b32f7063f`). An item that a wave ships after that commit can change state before the boundary. Each verb of D1 to D7 reads the item again.
- The age of an item is not a criterion in this review (F8.2).

## 9. Appendix: item lists

**D1, consume (18):** `arch-decisions-ad1-1-keep-the-process-name-in-each-read`, `builder-fence-erases-new-files-and-the-seat-says-pass-ac-1-console`, `cover-changed-subtracts-base-red`, `flake-e2e-fallback-raw-git-tempdir-cleanup`, `gate-contract-g1-registry`, `gate-contract-g4-base-red-attribution`, `landed-identity-zero-context`, `model-tier-policy-p1-agy-top-medium`, `resume-path-has-no-blocker-route`, `router-active-worktree-fleet-pin-lane`, `ship-refuses-a-hand-edited-generated-file`, `tmux-submit-verify-eight-rune-floor`, `triage-gate-refuses-the-answer-for-a-routed-pin`, `wave-integration-i0-deliverable-record`, `wave-integration-i3b-batch-test`, `g2-walker-fails-closed-before-enforcement`, `latency-wave-breach-from-wave-158`, `rebuild-pr-1254-acs-compile`.

**D2, narrow (19):** `cmd-evolve-cell-is-too-coarse-for-8-lanes`, `egps-regression-tia-selection`, `enforce-evidence-for-integration`, `intent-delta-gate-seam-threading`, `lane-floor-selects-the-tests-that-read-a-changed-data-file`, `lane-ship-checks-lint-and-derived-files`, `lane-test-floors-oversubscribe-the-host`, `ledger-restructure-c12-incremental-verify`, `model-tier-policy-p2-catalog-tier-pins`, `quota-pause-names-the-real-wall`, `sandboxed-seats-cannot-save-a-refreshed-login`, `ship-race-starves-a-slow-cycle`, `wave-integration-i11-plane-main-feeds`, `wave-integration-i7-recompose-after-console-merge`, `wave-integration-i8-status-verb`, `wave-orchestrator-o6-wire-oversight-into-the-loop`; parked: `edit-layout-c5-gate-lists-as-fragments`, `gate-child-env-allowlist`, `main-red-stop-provable-means-a-lane-can-prove-it`.

**D6, lane (35):**

| Group | Items |
|---|---|
| repair and fix (19) | `agy-tier-map-single-source`, `bridge-test-engine-fakes-sandbox-fence`, `bridge-tests-build-evolve-once`, `ci-macos-fixed-parallelism` |
| | `ci-test-integration-treewalk-parity`, `claude-2316-large-paste-submit-wedge`, `claude-cloud-driver-launch-confirmation-hardening`, `cloud-seat-has-its-own-builder-profile` |
| | `concurrent-resume-untriaged-scope-guards`, `core-tests-cut-git-starts`, `evalqualitycheck-fence-and-shadowing`, `idle-at-prompt-checkpoint-before-the-artifact-timeout` |
| | `inbox-claims-delta-followups`, `orchestrator-persona-decision-sections-propose-only`, `phaseobserver-test-hygiene`, `pr-refresh-test-measure-checkout` |
| | `premise-verification-anchor`, `stale-premise-reaches-build`, `sync-main-refreshes-the-index-with-the-merged-binary` |
| feature (8) | `arch-decisions-ad2-5-flip-inbox-curator-stage-to-enforce`, `harness-removal-test`, `loop-history-comment-removal-needs-an-archive-entry-lane`, `outcome-churn-revert-recovery` |
| | `recovery-retrospective-c5-judgment-todo-retire`, `shadow-contract-k1b-stage-actor-ports`, `shadow-contract-k3-stage-verify`, `shadow-contract-k5-stage-flip` |
| other (8) | `agy-docs-behind-the-code`, `cmd-cycle-wire-orchestrator-deps-decomposition`, `config-single-authority-sweep`, `docs-drift-sweep-2026-07` |
| | `effort-config-followups`, `fallback-lists-hold-only-latest-models`, `practice-claude-code-changelog-1bd0f3ce-202611140548`, `test-audit-t7-structure-pins` |

The per-item evidence is in the launch records of the 5 seats, under `runtime/.evolve/agent-runs/`, 2026-11-19:

| Seat | Run label |
|---|---|
| first pass, repair group | `agent-architecture-design-20261119T071938Z-f0a4` |
| first pass, feature group | `agent-architecture-design-20261119T071938Z-98c6` |
| first pass, other group | `agent-architecture-design-20261119T071938Z-1648` |
| adversarial pass, claims 1 to 20 | `agent-adversarial-review-20261119T073352Z-7518` |
| adversarial pass, claims 21 to 40 | `agent-adversarial-review-20261119T073352Z-ed62` |
