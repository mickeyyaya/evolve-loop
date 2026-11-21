# Plan: admission splits the coarse `cmd/evolve` cell

- **Status:** partly built, 2026-11-21 (K1 and K3 on main, and the operator step done; K2 not started). Earlier status: proposed, 2026-11-17.
- **Research:** [Console work: when the console builds an item and when it routes the item to a lane](../research/console-build-or-route-2026-11.md), finding F1.8.
- **Related:** [console-build-or-route-2026-11](console-build-or-route-2026-11.md), component K5. The package page: [internal-admission](../architecture/packages/internal-admission.md).

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Order](#7-order)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)

## 1. Request

The inbox item `admission-splits-the-coarse-cmd-evolve-cell`, plan K5:

> "Write the design first (docs-write, at least 3 approaches). Get the architecture review. File the build as a follow-up item. Do not change admission in this item."

A worked example. Item X edits `go/cmd/evolve/cmd_wave_status.go`. Item Y edits `go/cmd/evolve/cmd_inbox.go`. The two files do not share a symbol. Admission still holds Y until X ends, because both items edit the package `cmd/evolve`.

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F1 | A footprint holds package paths. It holds no file. Two todos conflict when their edited packages meet, or one edit meets the other reach. | `go/internal/admission/admission.go:153-159` |
| F2 | `Packages(files)` maps a `.go` file to its directory. Two files of `go/cmd/evolve` give one key. | `go/internal/admission/modulegraph.go:74-101` |
| F3 | `cmd/evolve` is a leaf. No package imports it. A leaf adds no reach. Its only conflict path is the rule "two edits meet". | `go/internal/admission/modulegraph.go:62-64`, `go/internal/admission/admission.go:197-207` |
| F4 | A todo with no file list gets its packages from the text. The text match finds only paths under `go/internal/`. A todo that names only `cmd/evolve` maps to no package and takes the hub cell. | `go/internal/admission/admission.go:240-253` |
| F5 | 19 to 22 waiting items named `cmd/evolve` among their cells in wave 177. | research F1.8 |
| F6 | The package `go/cmd/evolve` holds 1000 files in one Go package `main`. | `ls go/cmd/evolve \| wc -l`, 2026-11-17 |
| F7 | Three files take most of the edits in 30 days: `parallel_serial_tests_test.go` 91, `cmd_cycle.go` 53, `registry.go` 39. | `git log --since=30.days --name-only -- go/cmd/evolve`, 2026-11-17 |
| F8 | Each new verb adds one row to `var commands` in `registry.go`. | `go/cmd/evolve/registry.go:22` |
| F9 | The policy key `fleet.orchestrator.admission` selects the rule of the whole fleet. `enforce` uses `sharedEdit`. `shadow` uses `conflict`. Both rules call `sharedEdit`, so a file key in the shared-edit term applies in both modes. The live policy is `enforce`. | `go/internal/policy/fleet_orchestrator.go:154-156`, `go/internal/admission/admission.go:98-101,153-159` |
| F10 | Lanes integrate through the integration train. A merged build that does not compile is red there. | `evolve-pr N: integrate` commits |

*Implication of F1 to F3:* the one term "two edits meet" is the only term that holds `cmd/evolve` lanes apart. A change to that term for leaf packages leaves the reach rule alone.

*Implication of F7 and F8:* a file key does not free every item. Two items that both add a verb still share `registry.go`.

*Synthesis:* the shares of the 19 to 22 items that need `registry.go` or `parallel_serial_tests_test.go` are not measured. The Verification section measures them first.

## 3. Goals and non-goals

Goals:

1. Two items that edit two different files of `cmd/evolve` both admit.
2. Each existing admission invariant holds: hub capacity of one, family cells, the reach rule, and the property `TestAdmit_TwoAdmittedTodosNeverConflict`.
3. The change is reversible through policy data that names the leaf packages with file keys. The key `fleet.orchestrator.admission` is not that data (F9).

Non-goals:

- This item does not change `go/internal/admission/` or any caller.
- It does not fix text-only todos (F4). A better file list from Scout fixes them.
- It does not move verbs to new packages.

## 4. Candidate designs

### Approach A: file keys for leaf packages

For a leaf package and a todo with a file list, the edit key is the file, for example `cmd/evolve/a.go`. A todo with no file list for that leaf is "whole package". A whole-package edit conflicts with each file key of that leaf. Two file sets conflict only when they share a file.

- **For:** one term of one rule changes. A leaf has no reach, so the reach rule is intact. The `IsLeaf` method of the `Graph` port exists. The change works for each `cmd/*` leaf.
- **Against:** two lanes can still break the merged build through a shared symbol (F10). It does not help a text-only todo. It does not free two items that both edit `registry.go`.
- **Rejected variant:** a file key for each package. A non-leaf package has importers and reach. A file edit there can change an API that another lane compiles against.

### Approach B: one package for each verb

Move each verb into its own package under `internal/cli/<verb>cmd`, as `opscmd` and `guardcmd` are. Package cells then split the lanes.

- **For:** no admission change. It matches the existing pattern. It improves the module structure.
- **Against:** 1000 files must move (F6). Each move edits `cmd/evolve`, so each move serializes on the cell. Each new package is a non-leaf with reach. The wave does not gain from it for months.

### Approach C: a conflict rule that reads the edited symbols

Admission parses the symbols that each todo edits. Two todos conflict only on a shared symbol.

- **For:** the most precise rule in theory.
- **Against:** a todo names files, not symbols. The symbols are unknown before the build. Admission needs a parser, a guess or an LLM. Admission must stay a pure function of known facts.

### Approach D: verbs that register themselves

Each verb file adds its row to `commands` from its own `init` function. `registry.go` keeps the type and a sort. A new-verb item then edits only its new file.

- **For:** it removes the `registry.go` hot file (F8). The change is small and mechanical.
- **Against:** `init` order follows the file name, so the help output needs an explicit sort. Alone, it gains nothing, because the package cell still holds the lanes.

### Scores

Scale 1 (worst) to 5 (best). The weights add up to 1.0.

| Criterion | Weight | A | B | C | D alone |
|---|---|---|---|---|---|
| Meets the red test and the invariants | 0.35 | 5 | 4 | 2 | 1 |
| Fit with existing patterns | 0.20 | 5 | 4 | 1 | 4 |
| Small blast radius, reversible | 0.20 | 4 | 2 | 2 | 5 |
| Simplicity | 0.15 | 4 | 1 | 1 | 4 |
| Time to relieve the wave | 0.10 | 5 | 1 | 2 | 1 |
| **Weighted score** | 1.00 | **4.65** | **2.85** | **1.65** | **2.85** |

The scores are the judgment of this design. They are synthesis, not measurement.

## 5. Decisions

| Id | Decision | Fact | Rejected option and the fact that rules it out |
|---|---|---|---|
| D1 | **Chosen approach: A.** A leaf package gets file keys. Approach D follows as a staged complement. | F1, F3 | B: F6 and F8 make it too slow. C: F4 shows that a todo does not name symbols. |
| D2 | The footprint keeps package keys for non-leaf packages. It adds a file set for each leaf. A leaf with no file set is "whole package". | F3 | A file key for every package: a non-leaf has reach. |
| D3 | Two footprints share a leaf edit when one side is "whole package" for that leaf, or the file sets meet. | F4 | A rule on file sets only: a text-only todo passes a file todo and can break it. |
| D4 | `cells()` shows `cmd/evolve/<file>`, so `evolve wave supply` names the real holder. | F5 | A hidden key: the operator cannot see why an item waits. |
| D5 | The safety net for a shared symbol is the integration compile. A break is a red on the train. The measure is in Verification. | F10 | A compile probe in admission: it puts a build in a pure function. |
| D6 | The rollout uses one new policy list, `fleet.orchestrator.file_cells`. It holds the leaf packages that get file keys, for example `["cmd/evolve"]`. An empty list gives package keys, as today. The list is configuration data, not a flag. K1 ships with the list empty. Operator step: add `cmd/evolve`. | F9 | The key `fleet.orchestrator.admission`: it switches optimistic admission for the whole fleet, and file keys apply in both modes. A boolean flag: the project adds no feature flag. |
| D7 | Approach D is a separate item. It follows A after the measure shows `registry.go` as the main holder. | F7, F8 | D first: it gains nothing while the package cell holds. |

**Red test:** `TestAdmit_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit`. The graph is leaf-aware. Todo X lists `go/cmd/evolve/a.go`. Todo Y lists `go/cmd/evolve/b.go`. Both admit.

Guard tests, written beside the red test:

| Test | Setup | Result |
|---|---|---|
| `TestAdmit_TwoTodosThatEditTheSameFileOfALeafPackageHoldTheSecond` | X and Y list `go/cmd/evolve/a.go` | Y waits |
| `TestAdmit_AWholePackageEditOfALeafHoldsAFileEditOfThatLeaf` | X is text-only and names `cmd/evolve`; Y lists one file | Y waits |
| `TestAdmit_TwoTodosThatEditDifferentFilesOfANonLeafPackageHoldTheSecond` | two files of one non-leaf package | Y waits |

`TestAdmit_TwoAdmittedTodosNeverConflict` stays green.

## 6. Components

| Id | Component | Red test first | Lane or owner |
|---|---|---|---|
| K1 | File sets for leaf packages in the footprint, the shared-edit rule of D3 and the cell names of D4. Item `admission-file-cells-for-leaf-packages`. | the red test of section 5 | lane |
| K2 | Verbs that register themselves (approach D). Item `cmd-evolve-self-registering-verbs`. | `TestHelpOutputIsUnchangedWhenVerbsRegisterThemselves` | lane, after K1 shows its gain |
| K3 | The update of `docs/architecture/packages/internal-admission.md` for the file keys. It ships with K1. | none (a document) | lane |

The item files for K1 and K2 are in the inbox with this plan.

## 7. Order

1. K1 first, with `file_cells` empty. Behavior is unchanged. Then the operator sets `file_cells` to `["cmd/evolve"]`. The loop records which items admission frees.
2. Read the Verification numbers after one wave.
3. Keep `file_cells` as set when the integration reds do not rise. Add other leaf packages one by one.
4. K2 when the numbers show `registry.go` as the main holder.

## 8. Verification

| Measure | Command | Target |
|---|---|---|
| Waiting items held only by the `cmd/evolve` cell | `evolve wave supply` | fewer than 5, from 19 to 22 |
| Waiting items that also list `registry.go` or `parallel_serial_tests_test.go` | `evolve wave supply` | counted before K1, to size the gain |
| Integration reds with `redeclared` or `undefined` in `cmd/evolve` | `evolve ci classify` on the train runs | 0 for each week |
| Lane utilization | `evolve wave status` | at least 0.6, as in K4 of the console plan |
| Property of admission | `go test -count=1 ./internal/admission/...` | N/N PASS |

## 9. Risks

| Risk | Early signal | Remedy |
|---|---|---|
| Two lanes add the same top-level name in two files. | an integration red with `redeclared` | empty `fleet.orchestrator.file_cells`. Admission uses package keys again. The value of `fleet.orchestrator.admission` stays `enforce`. |
| The gain is small, because most items edit `registry.go`. | the count in section 8 | file K2 at once. |
| A text-only todo still takes the hub cell. | no change in its wait | a better file list from Scout. This plan does not fix it. |
| Scope creep into `go/`. | `git diff --name-only` shows a `go/` path | this cycle changes `docs/` and the inbox only. |

## 10. Open questions

1. How many of the 19 to 22 items name a file other than `registry.go` and `parallel_serial_tests_test.go`? Section 8 gives the first count. The answer decides how soon K2 must land.
2. Must `parallel_serial_tests_test.go` split by area? It takes 91 edits in 30 days. A test file adds no reach, but two edits of it still conflict under D3.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| K1 file sets for leaf packages | done, 2026-11-18 | cycle 2471 [`295d242`](https://github.com/mickeyyaya/evolve-loop/commit/295d24204) (`go/internal/admission/admission.go`, `go/acs/cycle2471/predicates_test.go`) | none |
| K1 operator step: `fleet.orchestrator.file_cells = ["cmd/evolve"]` | done, 2026-11-21. The operator approved it on 2026-11-21. | [#1606](https://github.com/mickeyyaya/evolve-loop/pull/1606) [`3e6ee1b`](https://github.com/mickeyyaya/evolve-loop/commit/3e6ee1b87) in batch PR [#1618](https://github.com/mickeyyaya/evolve-loop/pull/1618) (`.evolve/policy.json:134`) | none |
| K2 self-registering verbs | not started: no `TestHelpOutputIsUnchangedWhenVerbsRegisterThemselves` in `go/` | none | cmd-evolve-self-registering-verbs |
| K3 `internal-admission.md` update | done, 2026-11-18 | cycle 2471 [`295d242`](https://github.com/mickeyyaya/evolve-loop/commit/295d24204) (`docs/architecture/packages/internal-admission.md:107`) | none |

K1 and K3 are on main and the list is set. The first wave with the list set has not run in the record that I read, so the Verification numbers are not yet measured. K2 waits for those numbers and has an open item. The item `admission-file-cells-for-leaf-packages` is consumed. The older item `cmd-evolve-cell-is-too-coarse-for-8-lanes` stays open, and the numbers decide if it closes.
