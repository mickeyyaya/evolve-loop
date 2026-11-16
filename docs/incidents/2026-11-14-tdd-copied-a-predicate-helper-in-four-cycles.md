# 2026-11-14: the TDD seat copied a predicate helper, and the handoff floor failed four cycles

Related documents: the package note [internal-testdup.md](../architecture/packages/internal-testdup.md), the predicate kit guide [go/acs/README.md](../../go/acs/README.md) and the earlier record [2026-11-09-duplicate-ratchet-misses-staged-predicates.md](2026-11-09-duplicate-ratchet-misses-staged-predicates.md).

## Summary

- **Impact:** 4 of the 25 failed cycles in the Signal Center window (cycles 2270 to 2384, 102 sealed) have one cause. They are cycles 2331, 2337, 2349 and 2365. That is 16% of the failures. Each cycle stopped at the TDD phase with the verdict PASS.
- **Root cause:** every predicate file needs a small wrapper around `acsassert.ModuleDir`. The kit does not supply it, so each TDD seat writes its own copy. The duplicate check flags the copy.
- **Why one correction did not converge:** the correction text said "or the existing helper". The only existing helper was in the test package of an earlier cycle. TDD cannot call it. The text also named no function.
- **Fix:** design review gave the fix in PR #1467 after three design rounds. The kit supplies the wrapper, and the floor names the exact remedy.
- **State of main:** the fix is not on `main` at the commit `f86b7a1cb`. This record checked `go/pkg/acsassert` and `go/internal/testdup` there, and found no `GoModuleDir` and no `Wraps`. The section "What changed" lists the design, not a merged change.

## Terms used throughout

- **The predicate kit:** the helpers in `go/pkg/acsassert/predicate_kit.go`. A predicate file calls them and does not copy them.
- **The handoff floor:** the check of the host review gate that reads a TDD handoff. It rejects the handoff with a correction text.
- **A helper body copy group:** two or more sites with an equal helper body after normalization. `internal/testdup` finds them.
- **The wrapper:** a helper with two statements that calls `acsassert.ModuleDir` with a `testing.T` and stops the test on an error.

## Timeline

| Time (UTC) | Event | Evidence |
|---|---|---|
| 2026-10-27 | The TDD persona names the predicate kit: "call each, never write a copy". | `agents/evolve-tdd-engineer.md:401` |
| window start to 2384 | The Signal Center seals 102 cycles from 2270 to 2384. 25 of them fail. | Signal Center, cycles 2270-2384 |
| not recorded | Cycles 2331, 2337, 2349 and 2365 each end with "tdd aborted after verdict=PASS". | review gate reason in each cycle (below) |
| 2026-11-14 | The design review finds the root cause. It corrects the first idea that the seat ignores the kit. | design rounds of PR #1467 |

The review gate reason is the same in the four cycles: "phase tdd deliverable rejected after 1 correction(s): tdd handoff floor: 1 helper body copy group(s) ... call pkg/acsassert or the existing helper instead of a copy". The fact sheet gives no time for each failure, so the table gives the date only.

## Root cause

**F1.1** The persona already points the TDD seat to the kit (`agents/evolve-tdd-engineer.md:401`). It lists `ModuleDir`, `OutputTail`, `FuncCoverage` and `WalkModule`.
*Implication:* the seat does call the kit. A fix that only adds more persona text does not address the cause.

**F1.2** `acsassert.ModuleDir(start string)` returns `(string, error)` (`go/pkg/acsassert/predicate_kit.go:15`). A predicate needs a `string` and a stop on error. Each predicate file therefore wraps the call in a helper with two statements. Two examples exist: `moduleDir` in `go/acs/cycle2323/predicates_test.go:21` and `moduleDir` in `go/acs/cycle2328/predicates_test.go:32`. `go/acs/cycle2320/predicates_test.go:33` holds the sibling helper `goDir`.
*Implication:* the kit lacks the one wrapper that every predicate file needs.

**F1.3** The duplicate check skips a function with fewer than 2 statements (`minStatements = 2`, `go/internal/testdup/testdup.go:25`, used at `testdup.go:156`). The wrapper has exactly 2 statements, so the check flags it.
*Implication:* a wrapper that is correct by itself is a copy to the floor.

**F1.4** The correction said "call pkg/acsassert or the existing helper". The existing helper sat in the test package of another cycle, for example `acs/cycle2323`. A Go test package cannot import another test package. The text named no function either.
*Implication:* the seat cannot follow the correction. One correction never converged, and the cycle failed.

**F1.5** The sequence: the seat writes the wrapper and the floor flags it. The correction gives no usable remedy. The seat sends the same copy again, and the phase aborts.

## Decisions

The design in PR #1467 has these decisions. The fact sheet gives no ids, so they are numbered here.

| Id | Decision | Because | Rejected option and why |
|---|---|---|---|
| D1 | The kit supplies `acsassert.GoModuleDir(tb testing.TB)` and a `TestingForms` map `{ModuleDir: GoModuleDir}`. The map is the one home of the pair. | F1.2, F1.3 | Keep the wrapper in each predicate file. The floor flags it (F1.3). |
| D2 | `testdup.Group.Wraps` names the kit calls that a copy wraps. | F1.4 | Report only "a copy". The seat then has no function to call. |
| D3 | The floor parses the kit of the worktree, with no import, and names the exact remedy: "call acsassert.GoModuleDir", or "inline it, or call acsassert.X directly". | F1.4 | Import the kit into the floor. The floor then reads the kit of the host, not the kit of the cycle. |
| D4 | The text "call the existing helper" appears only when every site is in the cycle directory. | F1.4 | Keep the old text for all cases. It is wrong for a helper in another cycle. |
| D5 | The persona links the kit list in the README, not a second copy of the list. | F1.1 | Add a second list to the persona. It drifts from the README. |

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| D1 to D5 as above | #1467 | not verified in this worktree |

This record cannot verify PR #1467, the names `GoModuleDir`, `TestingForms` and `Wraps`, or a regression test name. None of them exists on `main` at `f86b7a1cb`. Add the test names to this table when the pull request lands.

## Limits

- The fix covers the wrapper around `ModuleDir`. A different two-statement helper that the kit does not supply still fails the same way. The floor names the kit call that the helper wraps, when one exists.
- The count of 4 of 25 comes from the Signal Center for one window. This record did not recompute it.

## What we learned

- A correction must name the exact action that the seat can take. "Or the existing helper" gave no action, and one correction did not converge.
- A check must test that its remedy is reachable from the place of the offender. A helper in another test package is not reachable.
- If a rule says "never write a copy", the kit must hold the helper that every case needs. Otherwise the rule forces a copy. The guard is the kit entry in D1 and the exact remedy in D3.
- The first diagnosis ("the seat does not use the kit") was wrong. The design review checked the persona and the code before it set the cause.
