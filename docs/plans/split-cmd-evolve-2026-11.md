# Plan: split cmd/evolve into verb-group packages along change coupling

- **Status:** partly built, 2026-11-21 (2 of 6 steps on main). Earlier status: proposed, 2026-11-17.
- **Parent plan:** `concurrent-edit-layout-2026-11` (PR #1545, commit `0ea7f8c1b`). Its component C6 asks for this design. Its decision D5 sets the rule of the move. Its open question Q2 is answered in section 5.
- **Related:** component C7 of the parent plan splits the hot package notes along the same groups.

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Census](#3-census)
4. [Candidate groupings](#4-candidate-groupings)
5. [Decision](#5-decision)
6. [Move order](#6-move-order)
7. [Red test for each move](#7-red-test-for-each-move)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Rejected](#10-rejected)

## 1. Request

The inbox item `edit-layout-c6-split-cmd-evolve-design` has three acceptance lines:

> 1. the design scores at least 3 groupings and names the first group to move
> 2. the docs pass ste-lint --strict and docs review
> 3. one build item for each group is filed

A worked example: two lanes edit `cmd_inbox_rank.go` and `cmd_wave_supply.go`. Both files live in `go/cmd/evolve`, so the package is one admission cell. The second lane waits for the first lane. After the split, the files live in two packages and both lanes run.

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F1 | `go/cmd/evolve` is one `package main`. It holds about 1,000 files, and 285 of them are not tests. | `ls go/cmd/evolve`, 2026-11-17 |
| F2 | Six verb groups already live in `go/internal/cli/`: `agentcmd`, `cloudcmd`, `contextcmd`, `guardcmd`, `opscmd`, `phasecmd`. | `go/internal/cli/` |
| F3 | The pattern is a thin shim in `main`. The shim body is one line, `return cloudcmd.Run(args, stdout, stderr)`. | `go/cmd/evolve/cmd_cloud.go:10` |
| F4 | A group that needs a function of `main` receives it through a `Deps` value. | `go/cmd/evolve/cmd_selfheal_propose.go:43-46`, `go/cmd/evolve/cmd_selfcheck.go:15-18` |
| F5 | `registry.go` lists each verb as `{Name, Summary, Run}`. Only the target of `Run` moves. | `go/cmd/evolve/registry.go:43-70` |
| F6 | The parent plan D5 says: split along change coupling, one group at a time, at a boundary, with no lane in the package. | parent plan D5 |

## 3. Census

The census has two measures. The first measure is the co-change of the files. The second measure is the coupling of each candidate group to the rest of `main`.

### 3.1 Co-change in 30 days

Command: `git log --since=30.days --no-merges --name-only --format='@@%h' -- go/cmd/evolve/`. A script groups each file by the token after `cmd_`. A test file counts with its source file. A file without the `cmd_` prefix belongs to CORE, except `root` files, which have their own row. Result on 2026-11-17: 302 commits.

| Group | Commits that touch it | Solo commits | Top partners (shared commits) |
|---|---|---|---|
| CORE | 183 | 42 | cycle 47, loop 41, wave 28, inbox 22 |
| cycle | 74 | 16 | CORE 47, loop 24, inbox 14 |
| loop | 74 | 18 | CORE 41, cycle 24, wave 14 |
| wave | 47 | 8 | CORE 28, loop 14, inbox 10, cycle 9 |
| inbox | 33 | 2 | CORE 22, cycle 14, loop 11, wave 10 |
| gc | 15 | 3 | CORE 12, loop 5 |
| integration | 12 | 5 | CORE 5, loop 4 |
| boundary | 12 | 2 | CORE 10, wave 7 |
| pr | 11 | 4 | CORE 5, inbox 2 |
| ci | 10 | 2 | CORE 7, root 1, cycle 1, inbox 1, wave 1 |
| quality | 11 | 2 | wave 8, CORE 6 |

Two facts follow. First, `cycle` and `loop` share 24 commits, so they are one cluster. Second, `boundary` and `quality` share 7 and 8 commits with `wave`, so they move with `wave`.

### 3.2 Coupling of the candidate groups

Method: a scratch package under `go/cmd` held the non-test files of one group. The command `go build -gcflags=-e` counted the distinct `undefined:` symbols. A second script counted the functions of the group that other `main` files name. The scratch package was deleted, and `git status` was clean.

| Group | Files | Outbound undefined symbols | Inbound functions |
|---|---|---|---|
| inbox | 21 | 12 | 9 |
| gc | 10 | 13 | 9 |
| wave | 11 | 47 | 5 |
| ci | 3 | 8 | 1 |
| pr | 6 | 7 | 6 |

An outbound symbol needs a `Deps` field or a move to a shared package. An inbound function needs an exported name or a `Deps` field on the `main` side.

## 4. Candidate groupings

The score uses four criteria, each from 1 to 5. The weights are: relief 0.35, move risk 0.25, fit with F2 0.20, simplicity 0.20. Relief means how many hot touches leave the `main` cell.

### Grouping A1: one package for each verb

About 80 packages, one for each verb.

- Relief 2. Most verbs have 0 to 2 touches in 30 days, so a move buys nothing. `cycle` and `loop` share 24 commits, so one item still holds two cells.
- Move risk 2. Each verb needs its own `Deps`. The 80 shims and 80 notes (C7) add work.
- Fit 3. The pattern of F2 holds, but at 13 times the scale.
- Simplicity 1.
- Score: 0.35·2 + 0.25·2 + 0.20·3 + 0.20·1 = 2.00.

### Grouping A2: co-change clusters, about 6 groups

The groups follow the shared commits of section 3.1:

| Package | Verbs |
|---|---|
| `inboxcmd` | inbox, inbox mover, curator |
| `landcmd` | pr, ci, integration, ship, land |
| `gccmd` | gc, worktree, sync-main, reset-sha |
| `wavecmd` | wave, boundary, quality, supply |
| spine (stays in `main` until last) | cycle, loop, CORE |

- Relief 4. The first four packages hold the groups with 33, 47, 15 and 11 touches, and each usual item stays in one cell. The touch counts overlap, so they are not added.
- Move risk 4. The sums per group are low except `wave` (47 outbound). The existing `Deps` pattern (F4) covers each seam.
- Fit 5. It reuses the `internal/cli/<group>cmd` shape of F2.
- Simplicity 4. Four new packages and four shims, then one design for the spine.
- Score: 0.35·4 + 0.25·4 + 0.20·5 + 0.20·4 = 4.20.

### Grouping A3: domain groups from the registry summaries

Groups follow the meaning of a verb: backlog, waves, cleanup, release, observability, setup.

- Relief 3. Meaning and co-change differ. `boundary` reads as release, but it shares 7 commits with `wave`. `quality` reads as observability, but it shares 8 commits with `wave`.
- Move risk 3. An item that touches both halves of a split pair lands in two cells.
- Fit 5. The pattern of F2 holds.
- Simplicity 4. About 6 packages.
- Score: 0.35·3 + 0.25·3 + 0.20·5 + 0.20·4 = 3.60.

### Score table

| Grouping | Relief 0.35 | Risk 0.25 | Fit 0.20 | Simplicity 0.20 | Score |
|---|---|---|---|---|---|
| A1 verb by verb | 2 | 2 | 3 | 1 | 2.00 |
| A2 co-change clusters | 4 | 4 | 5 | 4 | 4.20 |
| A3 domain groups | 3 | 3 | 5 | 4 | 3.60 |

## 5. Decision

D1. Use grouping A2. It has the highest score, and it is the only option where the files of one usual item stay in one cell. Fact F2 and fact F3 make it a copy of a proven pattern.

D2. The first group to move is `inbox`. Reasons from the census:

| Candidate | Touches | Outbound | Inbound | Reading |
|---|---|---|---|---|
| inbox | 33 | 12 | 9 | high relief and a `Deps` of 12 fields |
| gc | 15 | 13 | 9 | half the relief for the same cost |
| wave | 47 | 47 | 5 | most relief, but 47 symbols to inject |
| pr and ci | 11 and 10 | 7 and 8 | 6 and 1 | cheap, but little relief |

`inbox` has the best ratio of relief to risk. Its 12 outbound symbols fit one `Deps` value, as `agentcmd.Deps` does.

D3. **Answer to Q2 of the parent plan.** `cmd/evolve` splits into a few groups, not into many verb packages. Grouping A1 scored 2.00 against 4.20, and the census shows that verbs which change together must share a cell.

D4. The spine (`cycle`, `loop`, CORE) stays in `main` at first. CORE has 183 touches and couples to each group. The spine gets its own design after the four moves. If the main cell stays hot after them, the admission option K5 of the parent plan is the fallback.

D5. The package notes follow the groups. Each build item files its note split (C7) with the move.

## 6. Move order

Each move is one build item. Each item runs at a wave boundary. The command `evolve wave supply` must show no open lane with a file in the group. A move never starts when a lane holds a file of its group.

| Step | Item | Package | Depends on |
|---|---|---|---|
| 1 | `split-cmd-evolve-inboxcmd` | `internal/cli/inboxcmd` | none |
| 2 | `split-cmd-evolve-landcmd-pr-ci` | `internal/cli/landcmd` | step 1 |
| 3 | `split-cmd-evolve-gccmd` | `internal/cli/gccmd` | step 2 |
| 4 | `split-cmd-evolve-wavecmd` | `internal/cli/wavecmd` | step 3 |
| 5 | `split-cmd-evolve-landcmd-rest` | `internal/cli/landcmd` | step 4 |
| 6 | `split-cmd-evolve-spine-design` | design only | step 5 |

The build agent filed the six items as the requests `inbox-add-1` to `inbox-add-6` in the run directory of cycle 2460. Request `inbox-add-N` files step N of the table.

Step 2 is a warm-up of the `landcmd` group with the two cheapest verbs. Step 5 adds the other `landcmd` verbs. Step 4 comes late because it needs 47 injected symbols.

## 7. Red test for each move

A red test fails before the move and passes after it. Each move has two checks.

| Step | Red test | Why it fails before |
|---|---|---|
| 1 | `TestInboxCmdRun` in `internal/cli/inboxcmd` calls `inboxcmd.Run` and checks the exit code of `inbox list --help` | the package does not exist, so the test does not compile |
| 2 | `TestLandCmdRun` in `internal/cli/landcmd` for `pr` and `ci classify` | same |
| 3 | `TestGcCmdRun` in `internal/cli/gccmd` | same |
| 4 | `TestWaveCmdRun` in `internal/cli/wavecmd` | same |
| 5 | `TestLandCmdRunRest` in `internal/cli/landcmd` for `integration` and `ship` | same |
| 6 | no test, because the step is a design | the design passes `evolve docs ste-lint --strict` |

Each move also needs these checks, which fail while a file remains:

1. A test lists `go/cmd/evolve/cmd_<group>*.go` and expects only the one shim file.
2. The `cli_contract` cases for the verbs of the group pass through the shim with no change.
3. `gofmt -l .`, `go vet ./...` and `go test -count=1 ./cmd/evolve/... ./internal/cli/<group>cmd/...` all pass.

A test that checks file names alone is easy to game. The contract cases of check 2 prove that the behavior is the same.

## 8. Verification

| Measure | Before | Target after step 4 | Command |
|---|---|---|---|
| Items that wait for the `cmd/evolve` cell | read at the start of step 1 | fewer than at the start | `evolve wave supply` |
| Non-test files in `go/cmd/evolve` | 285 | under 220 (synthesis: the four groups hold about 50 files) | `ls go/cmd/evolve` |
| Commits that touch only a moved group | 0 | the groups leave the `main` count | the command of section 3.1 |

The first item records its own before value, because no source gives it today. A step is done only when its red test and the three checks of section 7 pass.

## 9. Risks

| Risk | Mitigation |
|---|---|
| The census drifts between this design and a move. | Each item runs the census commands again and compares. |
| Inbound callers break when a function moves. | Each item lists its inbound functions and moves them behind a `Deps` value. |
| A move meets an open lane. | The boundary rule of D5 and the check of `evolve wave supply`. |
| CORE keeps the main cell hot. | D4: a separate design for the spine, and K5 as the fallback. |
| The parent plan PR #1545 is not on main. | This plan links the plan by name and by commit. |

## 10. Rejected

- **One big move of every group.** It conflicts with each open lane in the package, and a review cannot hold the whole diff (parent plan D5).
- **A new `evolve` verb for the census.** The census stays a documented command in section 3 until a second user needs it.
- **A change of `registry.go`.** Only the target of `Run` moves, so the registry keeps its shape (F5).

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| Step 1 `inboxcmd` | done, 2026-11-18 | cycle 2503 [`5ed6d76`](https://github.com/mickeyyaya/evolve-loop/commit/5ed6d7674) (`go/internal/cli/inboxcmd`, `go/acs/cycle2503/predicates_test.go:73`) | none |
| Step 2 `landcmd` for pr and ci | done, 2026-11-21 | cycle 2564 [`ca8b6f0`](https://github.com/mickeyyaya/evolve-loop/commit/ca8b6f025) (`go/internal/cli/landcmd`) | none |
| Step 3 `gccmd` | not started: no `internal/cli/gccmd` package | none | split-cmd-evolve-gccmd |
| Step 4 `wavecmd` | not started | none | split-cmd-evolve-wavecmd |
| Step 5 `landcmd` rest | not started | none | split-cmd-evolve-landcmd-rest |
| Step 6 spine design | not started | none | split-cmd-evolve-spine-design |

Steps 1 and 2 are on main. Steps 3 to 6 each have an open inbox item. Every unfinished step has an item, so no follow-up draft is needed. The plan names `TestLandCmdRun`, but `go/` has no test of that name. The package uses `landcmd_run_test.go`, so the exact name is unverified.
