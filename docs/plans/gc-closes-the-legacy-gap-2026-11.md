# Plan: gc removes each worktree and branch that nothing needs

- **Status:** proposed, 2026-11-09. Round 2, after the architecture review and the docs review of round 1 (section 5 lists each finding and its fix). The review comes before any build.
- **Design:** [internal-gc](../architecture/packages/internal-gc.md), [internal-gcpolicy](../architecture/packages/internal-gcpolicy.md).
- **Evidence:** the manual cleanup of 2026-11-09, with its manifests in `backups/session-2026-11-06/cleanup-2026-11-09/` of the hub.

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

- **Dev worktree.** A worktree under `dev/` of the hub. The console and the operator make them.
- **Lane worktree.** A worktree under `runtime/.evolve/worktrees/`. The loop makes one for each cycle.
- **Keep reason.** The text that gc prints when it does not release an item.
- **Proof of recovery.** A saved copy from which an operator can restore an item. It is a ref in a git bundle, or a patch with an archive of the untracked files.
- **Live item.** An item that a process, an open pull request or a paused cycle still needs.

## 1. Request

On 2026-11-09 the operator asked: "I would like to improve gc to cover why it didn't do you just did in each check. Investigate why it doesn't. clean up those legacy branches and worktrees like you just did".

The console had cleaned the hub by hand. It removed 261 worktrees and 998 local branches. One `evolve gc --dry-run` just before had released none of them.

## 2. Facts

**F1** The dry run of 2026-11-09 kept all 217 dev worktrees. The keep reasons were: "holds ignored files" 140 times, "active within the 14-day limit" 54 times, "detached HEAD; no branch keeps its commits" 21 times, and "process runs inside" 2 times.
*Implication:* three gates hold 215 of the 217 trees, and only two of the 217 had a live reason.

**F2** `gateLeftovers` keeps a tree that holds any ignored file outside `devRebuildableIgnored` (`go/internal/gc/devleftovers.go:16-24`, `:39-46`). The 140 trees held only the output of the pipeline itself, for example `.commit-gate/attestation.json`, `.evolve/ledger.jsonl`, `.evolve/agent-runs/*.json` and `.evolve/guards.log`.
*Implication:* the list of rebuildable files does not know the files that evolve writes in each tree.

**F3** `landed` accepts a branch in three cases only (`go/internal/gc/devworktrees.go:328-346`). Its head is an ancestor of `origin/main`, a squash of the branch lands in main, or the branch has no net change against main. The train lands a pull request as a new integration commit (`evolve-pr <n>: integrate`). The inventory found 41 dev trees whose pull request was merged, and `git cherry` still showed their commits as not in main.
*Implication:* gc reads a merged pull request as work that is not in main.

**F4** `gateBranch` keeps each detached tree (`go/internal/gc/devworktrees.go:225-230`). Most of the 21 were review trees, for example `dev/cl-cr-1275`, whose head was the head of a pull request.
*Implication:* a detached head on main or on a known ref holds no work that a removal can lose.

**F5** A dirty lane worktree is released only when its cycle has a close-out dossier older than `salvage_after_hours` (`go/internal/gc/worktrees.go:339-348`, `:354-364`). `ClosedOutAt` reads `knowledge-base/cycles/cycle-<n>.json` or `.evolve/dossiers-pending/cycle-<n>.json` (`go/internal/dossier/cyclesdir.go:29-35`). Cycles 1836, 2008 and 2020 have neither file and no run directory, because run retention deleted them after 14 days.
*Implication:* the lane of an old cycle can never prove that its cycle ended, so gc flags it on each run, without an end.

**F6** `orphanBranchItems` deletes a `cycle-*` branch with no worktree in three cases only (`go/internal/gc/worktrees.go:403-433`). It is an ancestor of main, a squash with no net change, or empty. Ship rebases the cycle onto main before it lands. The dry run flagged 142 such branches as "unmerged orphan branch (no worktree)".
*Implication:* the branch of almost every shipped cycle stays forever.

**F7** No sweep of gc reads a local branch that is not `cycle-*` and has no worktree. The hub held 1,012 local branches before the cleanup. No sweep reads a remote branch. The remote held 638 branches after `git fetch --prune`, and 393 of them were merged into main.
*Implication:* branch growth has no owner.

**F8** The 157 items that the workspace sweep "flagged for manual review" go to no verb, no report and no inbox item. The same flags repeat on each run.
*Implication:* a flag without an owner is a keep without an end.

**F10** The loop runs gc only at the batch start and the batch end (`go/cmd/evolve/cmd_loop_prebatch.go:59`, `go/cmd/evolve/cmd_loop_batch.go:128`). The batch end follows the wave. That hook runs the run directories, the ledger seal and the lanes, and only plans the dev worktrees (`go/cmd/evolve/cmd_loop_outcome.go:74-103`). It does not stop orphan processes, trim the Go cache or sweep a branch. No gc step runs when one cycle ends.
*Implication:* on 2026-11-09, 781 orphan processes and 54.67 GB of Go cache waited for a manual `evolve gc`.

**F9** The manual cleanup kept 14 worktrees and lost no work. It saved a verified bundle of 2,907 refs and a patch with an untracked archive for each of the 94 dirty trees first. Then it kept only the core trees, the paused cycles, the trees of open pull requests and the trees with a live process.
*Implication:* a proof of recovery makes a removal reversible, so the gate needs only the live reasons.

## 3. Goals and non-goals

| Goal | Measure | Target |
|---|---|---|
| G1. gc releases each item that nothing needs | items that a manual cleanup removes and gc keeps, on the same hub | 0 |
| G2. No removal loses work | items without a proof of recovery at the time of removal | 0 |
| G3. A keep has a live reason | keep reasons outside the live list (process, open pull request, paused cycle, core tree, recent activity) | 0 |
| G4. A flag has an owner | flags that repeat on two runs without an operator report | 0 |

Non-goals:

- gc does not close a pull request. The operator closes it.
- gc does not stop a process that it did not start. The tmux leak has its own item, `tmuxtest-servers-outlive-their-test-binary`.
- This plan does not change the run directory retention.

## 4. Candidate designs

| Criterion | A. More evidence in each gate | B. Save first, then keep only the live items | C. A review queue |
|---|---|---|---|
| Closes F1 to F7 | partly: each gate needs new evidence | yes: one rule for each kind of item | no: it moves the work to the operator |
| Safety (G2) | each gate must be right | the saved copy covers a gate error | the operator decides each item |
| Size of the change | large: five gates change | medium: one save step and one live rule | small |
| The flags of F8 | stay | end | turn into a list |
| Score (1 to 5) | 2 | 5 | 2 |

**Design A** keeps each gate and adds evidence: pull request state for F3, reachability for F4, the ledger seal for F5. Each new check can be wrong in its own way. The 140 trees of F2 also need a second list of rebuildable files, and that list grows with each new file that evolve writes.

**Design B** saves a proof of recovery before each removal. Then the keep rule asks one question: does a live item need this? The gates of F2 to F6 answer a different question: can a removal lose work? The proof answers that question for all items at once. F9 shows the shape in practice.

**Design C** prints a list and asks the operator. The operator asked for gc to do the cleanup. C adds work to the operator.

B wins. It closes each gap with one rule, and its safety does not depend on each gate. The cost of B is disk for the saved copies, and a restore step that the operator must know.

## 5. Decisions

Each decision keeps the invariants of [internal-gc](../architecture/packages/internal-gc.md) section "Invariants", except where D9 amends one by name.

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | One recovery store: `.evolve/operator-salvage/<leaf>/`. gc adds a verified `refs.bundle` beside `HEAD`, `uncommitted.patch` and `untracked.tgz` (`go/internal/gc/salvage.go:40-66`). The dev archive writes the same layout. | F9; review R1.5 | A new store: `internal-gc.md` rejects a second store for the same job. `wtcheckpoint` only: a branch delete can remove its ref. |
| D2 | gc removes a tree or deletes a branch only after `git bundle verify` passes on the bundle of that item. A failed save or verify keeps the item. | G2 | Save and remove in one step: a failed save then loses work. |
| D3 | `gateLeftovers` and its nested repository guard stay. The rebuildable list gets the output of the pipeline: `.evolve/` and `.commit-gate/`, at any depth. | F2; review R1.3 | Drop the gate: `--exclude-standard` does not archive an ignored file, so a `.env` or a nested repository is lost. |
| D4 | A branch is landed when its pull request is merged and the branch tip equals the pull request `headRefOid`. gc reads `gh` once for each run. A closed pull request that is not merged is not landed. | F3; review R1.4 | Branch name only: a reused name or a later commit then reads as landed. |
| D5 | A detached tree is free of unique work when its head is an ancestor of `origin/main` or equals the `headRefOid` of a pull request. Else D1 saves the head in the bundle, and the idle rule applies. | F4 | Keep each detached tree: 21 trees stay without a reason. |
| D6 | A lane has ended when four facts hold. `isLive` is false, and `core.PausedLaneScopes` does not hold the cycle. `continuation-registry.json` does not bind its branch. The tree is older than `salvage_after_hours`. A close-out dossier is not needed. | F5; review R1.1 | "Below the oldest unsealed cycle": a quota-paused cycle holds no lease, and "sealed" has two meanings in the code. |
| D7 | Seven live reasons keep an item. They are a plane tree, a live process, a live run directory, a paused lane and a continuation binding. The last two are an open pull request at the tip, and activity within `archive_after_days`. A merged and clean item skips the activity rule, as `landedEarly` does today. | F1, F9; review R1.2 | Activity alone: 54 trees with merged work stay 14 days. |
| D8 | The cycle-end step of D12 sweeps only the items of its own cycle. The sweep of other local branches and the remote sweep run in an operator `evolve gc`, and at the wave end when `gc.wave_end.scope` is `full` (D13). | review R1.2 | One sweep for all: the hook must never touch the branch of a developer (`internal-gc.md` invariants). |
| D9 | Two invariants of `internal-gc.md` change. "An unmerged branch is never deleted" becomes "a branch is deleted only after D2, and only when it is ended (D6) or landed (D4)". The batch hook invariant becomes "the hook touches a dev worktree or a non-`cycle-*` branch only at `gc.wave_end.scope: full`, under D2 and D7". `internal-gc.md` gets an "Amended by" line that links this plan. | review R1.2, R3.2, docs review | A silent change: the spec of record then contradicts the code. |
| D10 | The remote sweep fetches first, then puts each remote tip in the verified bundle of D2. It deletes a remote branch that is landed (D4). It also deletes a branch that no D7 reason keeps, with activity measured from the commit time of the remote tip. It skips a branch that a repository ruleset protects. It deletes through the API, never through `git push`. The operator set the stage to `enforce` on 2026-11-09. | F7; review R1.4, R3.1; operator 2026-11-09 | A delete with no bundle: the commits of a closed pull request can exist only on the remote. |
| D11 | Each flag that repeats on two runs goes into the gc report with a count, and once into the inbox. | F8 | A silent flag: F8 shows that nobody acts on it. |
| D12 | When cycle N seals, gc runs for cycle N only. It runs when N is not paused, has no continuation binding and has no live process of its run. It stops the orphan processes whose `cwd` is in the lane of N. It saves and verifies (D1, D2), then releases the lane and the `cycle-N` branch. It does not wait for `salvage_after_hours`, `MinAgeMinutes` or `KeepRecent`. The seal proves the end. | F10; review R3.3; operator 2026-11-09 | Wave end only: a long wave keeps each finished lane until the end. The D6 age rule: it is for a lane with no seal, and a seal is a stronger proof. |
| D13 | At the wave end, the loop runs each sweep of `evolve gc`, with the same rules. The policy key `gc.wave_end.scope` selects `lanes` or `full`. This repository sets `full`. | F10; operator 2026-11-09 | Operator runs only: on 2026-11-09 the debt grew until a manual run. |

**Round 1 findings and their fixes.**

- R1.1 (CRITICAL, the ended rule) → D6.
- R1.2 (CRITICAL, continuation branches and the batch hook) → D7, D8, D9.
- R1.3 (CRITICAL, ignored files) → D3.
- R1.4 (HIGH, closed and merged) → D4, D10.
- R1.5 (HIGH, a third store) → D1.
- The docs review on F3 → F3 now names three cases.

**Findings on the amendment of 2026-11-09 and their fixes.**

- R3.1 (CRITICAL, X8 and the risk row did not match D10) → D10, X8 and the risk row now agree.
- R3.2 (CRITICAL, the hook scope at `full`) → D9, X6 and X9 name `gc.wave_end.scope`.
- R3.3 (CRITICAL, the cycle-end step and the D6 age rule) → D12 and X10 state the exact cycle-end rule.

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| X1 | The salvage store gets `refs.bundle` and a verify, and the dev archive writes that layout. `evolve salvage list` shows the bundle. | a removal whose bundle verify fails keeps the tree and the branch | console |
| X2 | The landed source from `gh`: merged pull requests with `headRefOid`, read once for each run | a branch with a merged pull request and the same tip is landed; a later commit on it is not | console |
| X3 | The rebuildable list gets `.evolve/` and `.commit-gate/` at any depth | a tree that holds only `.commit-gate/attestation.json` passes the gate; a tree with `.env` or a nested repository stays | console |
| X4 | The detached rule of D5 and the live rule of D7 for dev trees | a detached tree on a merged head is released; a detached tree with a unique head is saved first | console |
| X5 | The ended rule of D6 for lanes | the dirty lane of an old cycle with no run directory is saved and removed; a paused lane and a continuation lane stay | console |
| X6 | The branch sweep. At the batch start, at a cycle end and at `gc.wave_end.scope: lanes`, it sweeps only `cycle-*` branches. At `full` and in an operator `evolve gc`, it also sweeps other local branches under D2 and D7 | at `lanes` the hook never deletes `feat/x`. At `full` it deletes `feat/x` when it is ended or landed, after D2. An open pull request keeps it | console |
| X7 | The flag owner of D11 | a flag that repeats on two runs files one item, not two | console |
| X8 | The remote sweep of D10, at `enforce` for this repository | a failed bundle verify keeps the remote branch. `enforce` deletes a landed branch, and an idle branch that no D7 reason keeps. A ruleset branch and a branch with an open pull request stay | console |
| X9 | `internal-gc.md` gets the two amended invariants of D9 and the new rules, with a link to this plan | the docs check finds the "Amended by" line, and the hook invariant names `gc.wave_end.scope` | console |
| X10 | The cycle-end step of D12, called where the cycle seals | a sealed cycle with a dirty lane and an orphan process is cleaned at once. A cycle that seals into a pause or a continuation keeps its lane and branch. A live lane of another cycle is not touched | console |
| X11 | The wave-end run of D13, with `gc.wave_end.scope` in the policy | at `full`, the batch-end hook runs each sweep of `evolve gc`; at `lanes`, it runs the sweeps of today | console |

Each component lands as one pull request with the composed floor and 100% coverage of each changed function.

## 7. Order

1. X1 first, because each later component removes items, and D2 needs the verified store.
2. X2 and X3, because the live rule reads the landed source and the gate.
3. X4 and X5, the worktree rules.
4. X6, the local branch sweep, with X9 in the same pull request.
5. X7, so the remaining flags have an owner.
6. X10 and X11, so the sweeps run without an operator.
7. X8 last, because it is the only sweep that other people can see.

## 8. Verification

- **Before:** the dry run of 2026-11-09 released 0 of the 261 worktrees and 0 of the 998 branches that the manual cleanup removed (F1, F7).
- **After:** a dry run runs on a restore of the hub from the bundle of 2026-11-09. It releases the 261 worktrees and 998 branches, except each item that D3 or D7 keeps with a named reason.
- **G2:** a test restores a removed tree and a removed branch from the salvage store, and the content matches byte for byte.
- **Safety:** the property test of `internal-gc.md` (`TestPlanWorktreesNeverTouchesLiveDirtyUnmerged_Property`) gets paused lanes and continuation lanes in its random population, and passes.
- **G4:** two runs on a hub with one unowned flag file one inbox item.

## 9. Risks

| Risk | Control |
|---|---|
| A bad bundle | D2: `git bundle verify` runs before each removal, and a failed verify keeps the item. |
| Disk use of the store | The bundle of 2,907 refs was 1.14 GB on 2026-11-09. The store keeps the salvage TTL of the policy. A bundle for each item holds only the refs of that item. |
| `gh` is down | X2 falls back to the git evidence of today and says so in the report. The remote sweep and the early archive do not run. |
| A paused cycle looks ended | D6 reads `core.PausedLaneScopes`, the run directory and the continuation registry. The property test covers each case. |
| A rebuildable pattern hides real work | D3 adds only the two directories that evolve writes. A nested repository and any other ignored file still keep the tree. |
| A remote branch of another person | D10: the bundle comes first, an open pull request or recent activity keeps the branch, and a ruleset branch is skipped. The deleted list goes into the gc report. |

## 10. Open questions

- **Q1, answered 2026-11-09.** The operator said: "Clean up the remote branches also, make sure gc will do the same to include them, gc should be invoked after the cycle / wave is completed". D10 is at `enforce`, and D12 and D13 add the invocation points. The console deleted the remote branches by hand on that day, after the bundle.
- **Q2.** How long does gc keep a bundle? This plan proposes the salvage TTL of the policy.
