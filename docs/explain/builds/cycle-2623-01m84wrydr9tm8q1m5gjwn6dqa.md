# Build Explanation — Cycle 2623

## Build Binding
- Cycle: 2623
- Base SHA: b3882878d12755d7b07beee84feb4772dfdb7437

## Summary
The `internal/core` test files now start 730 weighted git processes, not 866. A new guard test fails when the count rises above the recorded number.

## Rationale
Cycle 2577 tried to measure the whole core test binary. That run took 232 seconds, and its kernel-time ratio was flaky. A static count of git start call sites in the test source needs less than one second and has no timing noise. The guard follows the same idea as PR #1225, which cut the git starts of the ship tests.

## Changed Areas
- `go/internal/gitstarts/gitstarts.go` — adds `Count`, which weighs each way that a test file starts git, and `Verify`, which fails above a recorded count.
- `go/internal/gitstarts/gitstarts_test.go` — tests the weights, the scope of the count and the refusal of a broken recorded file.
- `go/internal/gitstarts/apicover_named_test.go` — names each exported symbol of the new package for the apicover gate.
- `go/.apicover-enforce` — enrolls `./internal/gitstarts` in the repo-wide apicover gate.
- `go/internal/core/git_starts_ceiling_test.go` — runs `Verify` on the core test directory from the core test binary.
- `go/internal/core/testdata/gitstarts.json` — records the count, 730, that the guard enforces.
- `go/internal/core/build_blocker_exit_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/build_floor_newtests_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/build_testaudit_check_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/comment_floor_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/continuation_console_adopt_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/cover_changed_floor_strict_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/cover_changed_floor_tagfilter_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/cover_changed_floor_tier_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/floor_entry_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/repo_contract_floor_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/ste_floor_test.go` — moves git fixtures to the cached template helpers.
- `go/internal/core/worktree_startref_test.go` — builds the seed repo and the bare origin from templates.
- `go/acs/cycle2623/predicates_test.go` — holds the acceptance predicates of the cycle.
- `.evolve/evals/core-tests-cut-git-starts.md` — holds the eval graders of the task.
- `docs/architecture/packages/internal-gitstarts.md` — documents the new package.
- `docs/architecture/packages/internal-core.md` — documents the guard, and the page now passes the strict STE lint.

## Design Decisions
- The count is static and uses the Go AST. A `gittest.Fixture`, `Bare` or `At` call weighs 5, and `Clone` weighs 1. A template helper weighs 0, because it copies a cached tree.
- Any `.Git(...)` call, and any call with the string argument `"git"`, weighs 1.
- The recorded count must equal the real count. A later regression then has no slack to hide in.
- `Verify` refuses a missing, unreadable, negative or field-less recorded file. A broken baseline cannot pass every count.
- The package is small and has one purpose, so the guard and the counter do not live in `internal/core`.
- Rejected: a whole-binary measurement, as in cycle 2577. It is slow and flaky.

## Verification
- `go test -count=1 -tags acs ./acs/cycle2623` passes all nine predicates.
- `go test -count=1 ./internal/core` and `go test -count=1 ./internal/gitstarts` pass.
- `evolve docs ste-lint --strict` reports 0 findings on both package pages.

## Compatibility
No exported symbol of an existing package changes. The moved fixtures keep the same repo contents as before. The tests that use them keep their assertions.

## Limitations
The static count weighs call sites, not run-time starts. A call site inside a loop counts once. The recorded number is a ceiling that tracks the source, not a measure of kernel time.
