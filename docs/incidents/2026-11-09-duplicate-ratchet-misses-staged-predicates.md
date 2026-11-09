# 2026-11-09: the duplicate ratchet missed the staged predicates of a cycle and turned main red

The operator asked: "resolve the design flaw and trigger the cycles that failed due to this design flaw."

Related documents: the spec [test-system.md §6.4](../architecture/test-system.md#64-the-duplicate-ratchet), the package note [internal-testdup.md](../architecture/packages/internal-testdup.md) and the plan [test-system-efficiency-2026-10.md](../plans/test-system-efficiency-2026-10.md) (component T5).

## Summary

- **Impact:** main CI was red from 08:24Z to 09:24Z and again from 16:10Z on 2026-11-09. Three cycles of wave 162 failed at ship or at build: 2240, 2242 and 2249. They used 513 LLM minutes.
- **Root cause:** the duplicate ratchet dates an `acs/` directory by the git index, not by the commits. The new predicate directory of a cycle is in the index but in no commit at ship. The ratchet calls it old and does not scan it.
- **Effect:** the ship gate of the cycle passed. The landing committed the directory, and from then on the ratchet scanned it on main. Main went above the baseline.
- **Fix:** the ratchet dates a directory by the tree of `HEAD`. A directory that `HEAD` does not hold is new. The TDD self-check also refuses a predicate file that copies a helper body, while TDD can still change the file.
- **Repair of main:** two copies of one helper body in `cmd/evolve` now call the shared helper. The count goes back to the baseline 334.

## Terms used throughout

- **The ratchet:** the structure test `TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline` and the package `internal/testdup` that it calls.
- **A copy:** one extra site of a helper body that is equal to another site after normalization.
- **A predicate directory:** `go/acs/cycle<N>/`. TDD writes it. After the TDD phase, no phase can change it.
- **Staged:** a file that is in the git index and in no commit yet.

## Timeline

| Time (UTC) | Event | Evidence |
|---|---|---|
| 06:38 | Cycle 2231 lands the ratchet with `count=334` and `date=2026-11-09`. | commit `2937a9464` |
| 08:24 | Cycle 2239 lands `acs/cycle2239`. Its helper `hasWords` has the body of `has` in `internal/integration/ghci_test.go`. Main CI is red. | commit `7452aeec4`, run `failure` at 08:24 |
| 09:24 | Cycle 2241 lands a change to `has` in `ghci_test.go`. The pair breaks, and main CI is green. | commit `fe9550c22`, run `success` at 09:24 |
| about 09:30 | Cycle 2240 ships from the base `7452aeec4`. The ratchet counts 335 and the ship gate fails. | `.evolve/runs/cycle-2240/ship-repocontract-scan.log` |
| 16:10 | Cycle 2250 lands `acs/cycle2250` with two copies: `onDisk` and `write`. Main counts 336. Main CI is red. | commit `9a2bbe0e1`, run `42652044939` |
| 18:15 | The builder of cycle 2242 tries to change `acs/cycle2250` to lower the count. The frozen check refuses it. | `HEALING_I10_VIOLATION` signal of cycle 2242 |
| 18:2x | Cycle 2249 fails its ship gate at 336. Cycle 2242 fails at build. | `.evolve/runs/cycle-2249/ship-repocontract-scan.log` |

## Root cause

**F1.1** `loadAcsAges` reads the tracked set with `git ls-files -- acs` (`go/internal/testdup/testdup.go:88`). That command lists the index. A staged file is in the index.
*Implication:* the new predicate directory of a cycle is "tracked" at ship.

**F1.2** `isOld` returns `!touched` for a tracked directory (`testdup.go:121-122`). A directory is touched only when a commit after the baseline date changes it (`testdup.go:92-99`). A staged directory has no commit.
*Implication:* the ratchet calls the new directory old and skips it.

**F1.3** The ship gate runs the ratchet in the lane worktree before the landing commit (`go/internal/phases/ship/ship.go:126`, `repoContractGateRoot` at `ship.go:192`). Measured on 2026-11-10: the base `6e739c76c` with the change of cycle 2250 staged gives `ok`. The same change committed (`9a2bbe0e1`) gives "336 copies, above the baseline 334". The same change untracked also gives 336.
*Implication:* only the staged state escapes, and the staged state is the state that the ship gate sees.

**F1.4** After the landing, no later cycle can remove the copy from its side in `acs/`. The frozen check refuses each change to another cycle's predicate directory. A lane can remove only the other site of the pair, as cycle 2241 did by luck at 09:24.
*Implication:* each escape turns main red until a lane changes the right file. Every ship fails until then.

**F1.5** The frozen predicates of cycle 2231 run in the durable ACS suite of CI (`go/acs/cycle2231/predicates_test.go`). They pin the API `Scan(root, since)`, the date rule for `acs/` (`TestC2231_004`), and the rule "the count equals the baseline" (`TestC2231_003`, `TestCheck_FailsWhenTheBaselineIsAboveTheCountSoALoweredCountLowersIt`).
*Implication:* a fix must keep the API, the date rule and the equal-count rule.

**F1.6** At ship, the cycle cannot change its own predicate directory either. The build floor skips it, and the persona of the TDD engineer says "you are the only gate on them" (`agents/evolve-tdd-engineer.md:100`).
*Implication:* a ratchet that catches the copy only at ship spends the whole cycle. The copy must be caught at the TDD self-check, where TDD can still change the file.

## Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | A directory with a change against `HEAD` is new (`git diff --relative --name-only HEAD -- acs`). The ship gate then counts the directories that main counts after the landing. | F1.1, F1.2, F1.3, R1 | Read the tracked set from `git ls-tree HEAD`. It closes the escape for a new directory. It misses a staged change in an old directory such as `acs/regression` (R1). Drop `acs/` from the scan: it breaks `TestC2231_004` (F1.5) and lets each new predicate file copy helpers again (test-system.md §6.5). |
| D2 | Add the host reviewer `tddPredicateFloor` in `internal/core`. `withMandatoryReviewers` puts it in the chain of every orchestrator. On a TDD handoff it rejects with retry each group of equal bodies that has a site under `go/acs/cycle<N>/`. | F1.6, R2, R3, R4 | Put the check in `evolve phase verify tdd`. The host does not run that code (R2). The rendered self-check passes no `--worktree`, so the check never runs (R3). Check only at ship or at the build floor: no phase after TDD can change the predicates (F1.6). |
| D3 | Repair main through the shared helper of `cmd/evolve`: `writeReplayFile` and `setupWrite` call `writeE2File`. The count goes from 336 to 334. The baseline stays 334. The package note records the two copies that stay in `acs/cycle2250`. | F1.4, F1.5, R5 | Raise the baseline to 336: the ratchet then accepts two copies that a lane can remove. Move the baseline date: it hides each copy of 2026-11-09, not only the two. Change `acs/cycle2250`: it is the frozen record of a cycle, and a durable suite reads it. |
| D4 | Add `testdup.GroupsUnder(report, dir)`. The floor reads the predicate directory from the cycle number, not from the handoff file list. | D2, R4 | Match the handoff `testFiles`. They are relative to the repository, and the sites are relative to `go/` (R4). A helper in a file outside the list also escapes. |

The architecture review of this design (agent run `agent-architecture-reviewer-20261109T185650Z-e907`) gave these refinements:

- **R1** A staged change in an old `acs/` directory that `HEAD` holds also escapes.
- **R2** (CRITICAL) The host TDD gate does not run the checks of `evolve phase verify tdd` (`go/internal/cli/phasecmd/phase_verify.go:117-128`).
- **R3** (CRITICAL) The rendered self-check `evolve phase verify tdd --workspace <dir>` (`go/internal/phasecontract/render.go:118-120`) leaves the worktree empty, so `verifyDeliverable` returns before each worktree check. The inbox item `tdd-self-check-worktree-checks-never-run` tracks this for `tdd_comment_floor` and `unreachable_frozen_pin`.
- **R4** (HIGH) The handoff files and the scan sites use two path roots.
- **R5** The two copies in `acs/cycle2250` stay, and the package note must say so.

## What changed

| Change | Regression test (red without the change) |
|---|---|
| D1 in `go/internal/testdup/testdup.go` | `TestScan_ScansAStagedAcsCycleDirectoryThatNoCommitHolds`, `TestScan_ScansAnOldAcsDirectoryWithAChangeThatNoCommitHoldsYet` |
| D2 in `go/internal/core/tdd_predicate_floor.go` and `orchestrator.go` | `TestTddPredicateFloor_TheOrchestratorSendsAPredicateThatCopiesAHelperBackToTdd` |
| D3 in `go/cmd/evolve` | the ratchet `TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline` on main |
| D4 in `go/internal/testdup/testdup.go` | `TestGroupsUnder_KeepsOnlyTheGroupsWithASiteInTheDirectory` |

The floor enforces at once, without a shadow stage. It fails only a cycle whose own predicates hold a copy, and the ship ratchet fails that cycle anyway after D1. The floor moves the same failure to the one phase that can fix it.

The failed cycles keep their work. Cycles 2240 and 2249 have continuation snapshots (`evolve continuation list`). The item of cycle 2242, `finder-fix-ownership-f5-deterministic-fast-path`, is open in the inbox. The next wave on a green main starts all three.

## Limits

- The ratchet compares one count for the whole tree. Two lanes that each add one site of the same new body pass alone and fail together on main. The integration train sees the merged tree first.
- The equal-count rule makes two lanes that each remove one copy conflict on `go/.test-dup-baseline`. The inbox item `testdup-equal-count-conflicts-across-lanes` tracks this.

## What we learned

- A gate that dates files must read the commits, not the index. The index holds the change that the gate judges.
- A gate on a file that only one phase can change must run in that phase. A later gate can only fail the cycle.
