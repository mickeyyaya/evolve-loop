# Research: the cell grain of `cmd/evolve` in lane admission

- **Date:** 2026-11-24. **Type:** research dossier. **Base:** `origin/main` 9bb753968.
- **Item:** `cmd-evolve-cell-is-too-coarse-for-8-lanes` (rank 25 of [feature-finish-line-2026-11](../plans/feature-finish-line-2026-11.md)).
- **Related:** [admission-cmd-evolve-cell-2026-11](../plans/admission-cmd-evolve-cell-2026-11.md), [internal-admission](../architecture/packages/internal-admission.md), [wave-orchestrator](../architecture/wave-orchestrator.md) section 6.

## Table of contents

1. [Request](#1-request)
2. [Terms used throughout](#2-terms-used-throughout)
3. [Findings](#3-findings)
4. [The three grains on the measured record](#4-the-three-grains-on-the-measured-record)
5. [Candidate approaches](#5-candidate-approaches)
6. [Decisions](#6-decisions)
7. [Branch cycle-cd3ae73e-2385](#7-branch-cycle-cd3ae73e-2385)
8. [Follow-up items](#8-follow-up-items)
9. [Limits and open questions](#9-limits-and-open-questions)

## 1. Request

The item states: "with one in-flight item holding the cell cmd/evolve, 12 lane-routable items waited, and the 8-lane fleet ran 4 lanes" (wave 172 census, 2026-11-14). It asks for a design that compares three grains on the conflict record of waves 170 to 173. The three grains are the package, the file set that the item declares, and the verb file family. It also asks for a decision on branch `cycle-cd3ae73e-2385`.

The answer first: the file grain exists on main and is switched on. This item needs no new admission code.

## 2. Terms used throughout

| Term | Meaning |
|---|---|
| Cell | The unit that admission locks for a lane. Admission computes it from the files of an item. |
| Package grain | One cell for each Go package. All of `cmd/evolve` is one cell. |
| File grain | One cell for each declared `.go` file of a listed leaf package, for example `cmd/evolve/cmd_gc.go`. |
| Verb family | The files of one verb. The first two `_` tokens of a `cmd_` file name give the family, for example `cmd_wave`. A `_test.go` file joins the family of its subject. |
| Leaf package | A package that no other package imports. `cmd/evolve` is a leaf. |
| `file_cells` | The policy list `fleet.orchestrator.file_cells`. It names the leaf packages that get file grain. |

## 3. Findings

**F3.1** File grain is built. A `.go` file of a listed leaf package becomes a file key. Any other path keeps the package key. Source: `go/internal/admission/admission.go:245-270`.
*Implication:* the item's red case exists in code.

**F3.2** The policy sets `admission: enforce` and `file_cells: ["cmd/evolve"]`. PR 1606 (`3e6ee1b87`) landed it on 2026-11-21. Source: `.evolve/policy.json:134-137`.
*Implication:* file grain runs in the live fleet.

**F3.3** The two tests that the item asks for pass on main: `TestAdmit_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit` and `TestAdmit_TwoTodosThatEditTheSameFileOfALeafPackageHoldTheSecond`. Source: `go/internal/admission/file_cells_test.go:35,56`. Two census tests cover the same cases: `go/internal/orchestrator/census_file_cells_test.go:16,49`.
*Implication:* a new red test for this item passes at once. It probes nothing new.

**F3.4** The item names `go/internal/orchestrator/admission.go`. That file does not exist. The admission code is in `go/internal/admission/admission.go`. Source: `ls go/internal/orchestrator/`.
*Implication:* the `files` field of the item must say `go/internal/admission/admission.go`.

**F3.5** Admission records no signal for a pair that file grain admits. `recordOptimistic` fires only when the reach rule holds a pair. Two file keys in one leaf share no edit and no reach. Source: `admission.go:133-143,157-159`.
*Implication:* nobody can compute a fail rate for file-grain pairs today.

**F3.6** Of 36 lanes in waves 170 to 173, 8 declared a `cmd/evolve` path. 5 of the 8 declared the directory (`go/cmd/evolve` or `go/cmd/evolve/`). Source: `inbox/` and `inbox/consumed/`, read on 2026-11-24.
*Implication:* a directory is a whole-package edit under each grain. File grain frees only items that name files.

**F3.7** Item `cmd-evolve-self-registering-verbs` (rank 28) removes the shared edit of `registry.go`. Today `registry.go` appears in 1 of 60 lane-routable `cmd/evolve` items. Source: live inbox, 2026-11-24.
*Implication:* rank 28 does not depend on this item and is not urgent.

## 4. The three grains on the measured record

### 4.1 Data used

| Source | Use | Limit |
|---|---|---|
| Live inbox on 2026-11-24: 60 lane-routable items that declare a `cmd/evolve` path | Count the pairs that each grain admits (1770 pairs) | Today's supply, not the supply of waves 170 to 173 |
| `.evolve/signals.ndjson`, 2026-11-13T17:50Z to 2026-11-15T07:53Z | Lane launches, optimistic admissions and their failures | The records measure the reach rule, not a file grain |
| `LOOP_ORCH_SUPPLY` signals, 2026-11-18 to 2026-11-23 | The `admissible` count before and after 2026-11-21 | Correlation only |

I found no conflict count for each wave. `.evolve/waves/wave-17x.json` holds only the run outcome. I do not use the "7 conflicts" figure of branch 2385, because I cannot verify it.

### 4.2 Admitted pairs

The count is exact. It counts declared files in the live inbox. It is not the result of a run.

| Grain | Admitted pairs of 1770 | Share |
|---|---|---|
| Package | 0 | 0% |
| Declared file set | 1016 | 57% |
| Declared file set and the id-family rule that `sharedEdit` already applies | 992 | 56% |
| Verb family (first two tokens) | 954 | 54% |
| Verb family and the id-family rule | 939 | 53% |
| Verb family by exact stem (a file and its test) | 1015 | 57% |

14 of the 60 items declare the whole directory. They hold every other item, so they remove 734 pairs. The 46 other items make 1035 pairs. 19 of those share a file. The verb family holds 62 more of the 1016 pairs (6%).

In waves 170 to 173 the same method gives 0 of 28 pairs at package grain and 1 to 3 of 28 at file grain. It gives 0 to 2 of 28 at verb-family grain. The range depends on one path: `composed-gates-test-only-the-affected-packages` lists `cmd/evolve/cmd_composition_wiring.go` with no `go/` prefix. `packageOf` maps only paths with the prefix `go/` (`go/internal/admission/modulegraph.go:86-100`). That path holds no cell. This range is an estimate, and the sample of 8 items cannot give a rate.

### 4.3 Fail rate of admitted pairs

| Grain | Admitted pairs measured | Failed | Fail rate |
|---|---|---|---|
| Package | 0 (no admission) | not applicable | not applicable |
| Declared file set | no record (F3.5) | unknown | unmeasured |
| Verb family | no record | unknown | unmeasured |
| Reference: optimistic admission of reach-disjoint pairs, waves 170 to 173 | 13 | 4 | 31% |
| Reference: the same signals, 2026-11-03 to 2026-11-22 | 211 | 43 | 20.4% |

The two reference rows come from `LOOP_ORCH_ADMISSION_OPTIMISTIC` and `LOOP_ORCH_OPTIMISTIC_ADMISSION_FAILED`. The four failures in the first row held the gates `test,apicover`. These rows are not file-grain rates. They show the rate of the rule that the fleet already runs. I do not estimate a file-grain fail rate.

### 4.4 Effect of the switch of 2026-11-21

The `admissible` count of the supply census was 7, 3, 12, 8 and 6 from 2026-11-18 to 2026-11-20. It was 16, 16, 15 and 21 after the switch. `LOOP_ORCH_SUPPLY_SHORT` last fired on 2026-11-20. This agrees with file cells that free items. It does not prove it: the window changed from 8 to 5 lanes on 2026-11-21, and the inbox changed.

## 5. Candidate approaches

| Id | Approach | For | Against |
|---|---|---|---|
| A | Keep the live file grain. Record the findings. Add no admission code. Retire branch 2385. | No risk. K1 satisfies the item (F3.1 to F3.3). | It leaves F3.5 open. Two follow-up items close it. |
| B | A, plus a record of file-grain pairs and a verb-family hold at stage `shadow`. | It gives the measure. | It adds a mechanism with no fail-rate evidence behind it. The verb family holds only 6% more pairs. |
| C | Reuse branch 2385: file grain for every leaf package with no list. | Already written. | It conflicts with main. It removes the `file_cells` switch. |
| D | Resolve a directory declaration into a file set. | It frees 14 of 60 items. | Admission must stay a pure function of known facts. A guess is a silent change. The author must name the files. |
| E | Give `cmd/evolve` a hub-like capacity of N. | Simple. | Two lanes can edit one file with no check. |

## 6. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Take approach A. The item is satisfied by K1. | F3.1, F3.2, F3.3 | B: no fail-rate evidence exists, and the verb family adds a 6% hold. C: it conflicts with main and drops the switch. D: it guesses. E: it drops the file test. |
| D2 | Do not add a verb-family stage now. | F3.5, 4.3 | A stage key with no record cannot be judged. Reopen it when follow-up `admission-records-file-grain-pairs` gives a fail rate. |
| D3 | Fix the declaration, not the grain, for items that name a directory. | F3.6 | Admission cannot know which files an item edits. |
| D4 | Correct the `files` field of the item to `go/internal/admission/admission.go`. | F3.4 | none |

Consequence: the lanes keep the 57% pair gain. Items that declare a directory still hold the cell. The risk of a shared symbol across two files stays unmeasured until follow-up 2 lands.

What changes the decision: a measured fail rate for file-grain pairs that the operator judges too high. Then reopen approach B.

## 7. Branch cycle-cd3ae73e-2385

Decision: retire the branch. Reuse nothing.

| Fact | Source |
|---|---|
| The branch has one commit, `6d06df8b9`, "salvage snapshot". It is 536 commits behind main. | `git log origin/main..cycle-cd3ae73e-2385` |
| A merge with main conflicts in `go/internal/admission/admission.go`. | `git merge-tree --write-tree` |
| The code (`editCells`, `editsMeet`, `fileMeetsPackage`) does what K1 does, with no policy list. | branch diff |
| Its tests and docs repeat the tests and docs of K1. | F3.3 |
| Its table "Conflicts removed 0 of 7" has no source that I can check. | branch doc |

`evolve gc` flags a branch only when it is an unmerged orphan with no worktree (`go/internal/gc/worktrees.go:575-590`). The flag ends when the branch is gone.

The retire path is the gc archive at the boundary. The boundary gc saves the branch head under the ref `refs/archive/salvage/<branch>`, here `refs/archive/salvage/cycle-cd3ae73e-2385`. Then it removes the branch. This follows the branch cleanup of 2026-11-19. This document and its author delete nothing. After the boundary run, `evolve gc` must not list the branch.

## 8. Follow-up items

| Item | What it does |
|---|---|
| `hub-items-name-their-files-at-intake` | An intake check for an item that edits a listed leaf package or a hub. It asks the author to name files, not the directory. It closes F3.6: 14 of 60 items declare `go/cmd/evolve`. It also rejects a `.go` path with no `go/` prefix. |
| `admission-records-file-grain-pairs` | A record of each pair that file grain admits, with its holder and its ship result. It closes F3.5, so the operator can read a fail rate. It needs its own design and a stage at `shadow`. |

## 9. Limits and open questions

- The pair counts use the live inbox of 2026-11-24, not the supply of waves 170 to 173.
- No fail rate exists for a file grain. The two reference rows are a yardstick only.
- The rise of `admissible` after 2026-11-21 is a correlation.
- Open: which fail rate must the operator accept for file-grain pairs? Follow-up 2 gives the data. The operator sets the number.
