# Build Explanation — Cycle 1732

## Build Binding
- Cycle: 1732
- Base SHA: c4ddce65df0e8a4035da997c80e863beee54a438

## Summary
The five oversized `internal/gc` functions — `Plan` (69 lines), `Apply` (55), `Discover` (57), `PlanWorktrees` (99), `ApplyWorktrees` (74) — are reduced to 45, 37, 43, 30 and 30 lines. Each one now reads as a sequence of named steps. Their five allowances are deleted from the size-ratchet offender list. Twelve new characterization tests pin 16 behaviors that the existing suite left unpinned. Behavior is unchanged: the new tests pass on both the baseline and the refactored sources.

## Rationale
The ratchet (`sizeratchet.MaxLines` = 50) only tolerated these functions through `offenders.json` allowances. Extract-method is the smallest change that clears them: each extracted block moves verbatim into an unexported helper in the same file, and the parent calls it. Every baseline comment stays in its origin file, so no *why* is lost and none is added. The five exported doc comments are untouched. Characterization tests came first because a mutation probe at the baseline found 16 observable-behavior mutants that the suite did not kill. Without those tests, a subtle slip during extraction would have gone unnoticed.

## Changed Areas
- `go/internal/gc/gc.go` — `Plan` now calls `planRunLadder` (Rule 1) and `planTrackerTTL` (Rule 2). The rule comments stay at the call sites. `Apply`'s archive case now calls `archiveItem`, which returns the error that `Apply` previously appended inline.
- `go/internal/gc/discover.go` — The dir-or-symlink stat branch of `Discover`'s loop moved into `runDirInfo(dir, e) (os.FileInfo, bool)`, along with its symlink comment.
- `go/internal/gc/worktrees.go` — `PlanWorktrees` is now five named steps: `mergedBranches`, `scanWorktrees` (the verbatim candidate loop), `eligibleAfterGrace` (the stat and MinAge grace tail), `removalItems` (KeepRecent) and `orphanBranchItems` (branch backlog). The local `eligible` type moved to package level. `ApplyWorktrees` is now three passes: `refuseChangedTargets` (pass 1, TOCTOU), `removeWorktrees` (2a) and `deleteBranches` (2b), followed by the unchanged trailing prune.
- `go/internal/gc/extraction_characterization_test.go` — A new test file with no comments. It has 12 tests (one of them table-driven with 3 cases) that kill all 16 mutants listed in the cycle's predicate 009.
- `go/internal/sizeratchet/offenders.json` — Deletes the five `internal/gc.*` allowances. They are removed, not lowered, as `LoadOffenders` requires for a function within the limit.
- `go/acs/cycle1732/predicates_test.go` — The TDD phase's 10 acceptance predicates for this lane. The Builder commits this file but did not author it.
- `.evolve/evals/sizeratchet-shrink-gc.md` — The TDD phase's eval (10 `score_cap` entries). The Builder commits this file but did not author it.

## Design Decisions
- The moved lines are verbatim apart from these parameter renames: in `planRunLadder`/`planTrackerTTL`, `opts.Runs` → `runs`; in `archiveItem`, `it.Path` → `path` and the inline `errs = append(...)`/`continue` → `return fmt.Errorf(...)`; in `runDirInfo`, `continue` → `return nil, false` and the reuse of the outer `err` → a local `info, err := e.Info()`; in `eligibleAfterGrace`, `e.branch` → `branch` and `continue` → `return eligible{}, false`; `PlanWorktrees`' `merged` error path returns a nil map rather than an empty one, but that value is never read on the error path. All 16 mutation anchors stay verbatim and occur exactly once.
- The rule helpers take `pol Policy` (not `RunsPolicy`), so `pol.Runs.*` / `pol.TrackerTTLDays` expressions move unchanged.
- The rule helpers receive the `add` closure, so the protected-path hard rule stays in one place (`Plan`'s `add`) and is not duplicated in each helper.
- Call order to the injected git runner is preserved exactly: `worktree list` → `branch --merged` → per-candidate `status` → `branch --list`. `ApplyWorktrees` aggregates errors in the same order: pass-1 refusals, then remove, then branch-delete, then prune.
- Alternative rejected: a two-stage "select candidates, then classify" split of the worktree loop. It would have reordered the `isLive`/`isDirty` evaluation across worktrees and rewritten every `e.branch` reference. A verbatim loop plus the `eligibleAfterGrace` tail keeps the move reviewable.

## Verification
- `go test -tags acs -count=1 -v ./acs/cycle1732` — 10/10 PASS. This includes 009 (16/16 mutants killed) and 007/010 (no comment added or deleted).
- `go test -count=1 -race ./internal/gc` — ok.
- The 12 new tests also pass when the three production files are overlaid with their `c4ddce65` versions (`go test -overlay`), so they pin pre-existing behavior rather than the refactor.
- `gofmt -l`, `go vet ./...` and module-wide `go test -count=1 ./...` were run before handoff (see build-report.md).

## Compatibility
There are no exported API changes: every new identifier is unexported, and signatures, doc comments, error texts and manifest ordering are unchanged. `offenders.json` loses five keys, which the ratchet test and `LoadOffenders` now require.

## Limitations
`scanWorktrees` is 43 lines. It is within the limit, but it is the densest remaining step. A later split (for example, a worktree-plan accumulator) is possible and is out of scope here. The characterization tests pin only the 16 selected mutants, not every possible behavior of the five functions.
