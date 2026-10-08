# Build Explanation — Cycle 1834

## Build Binding
- Cycle: 1834
- Base SHA: 8d6f2bbabb90a471842fbe1d2fc1808feaacba84

## Summary
The shared test-fixture package `internal/gittest` now retries a git exec once when it fails with EBADF or a closed pipe, and ship's test helper delegates to that one rule instead of keeping its own copy.

## Rationale
PR #753 (CI run 36719406746) failed on ubuntu-latest in `gittest: git init -q -b main ...: fork/exec /usr/bin/git: bad file descriptor`, a change that touched neither ship nor gittest. Cycle 249 had already given ship's own git runner a one-retry rule for the same race, but every other package's real-git tests run through `gittest.Repo.Git`, which had none. Moving the rule into gittest covers every fixture with one implementation; keeping it to exactly one retry on a fresh `exec.Cmd`, and only for EBADF or `io.ErrClosedPipe` found by `errors.Is`, keeps a real git failure from being masked.

## Changed Areas
- `go/internal/gittest/fixture.go` — adds the exported `CaptureWithEBADFRetry` and `IsEBADFLike`, and routes `Repo.Git` through the retry, building a fresh `exec.Cmd` per attempt through a package-level `combinedOutput` seam that the tests replace to inject exec faults.
- `go/internal/gittest/apicover_named_test.go` — names and covers the two new exports so gittest stays apicover-enforced.
- `go/internal/gittest/fixture_test.go` — the TDD phase's exec-fault tests: a transient first-exec fault in `Fixture` is retried once, a persistent EBADF fails the test naming `git init` and the error, and non-EBADF failures run once.
- `go/internal/phases/ship/ebadf_helpers_test.go` — ship's `captureWithEBADFRetry` now only returns `gittest.CaptureWithEBADFRetry`; ship's private `isEBADFLike` is removed.
- `go/acs/cycle1834/predicates_test.go` — the TDD phase's acceptance predicates for the shared rule, the no-masking classification, ship's delegation and the apicover gate.
- `.evolve/evals/gittest-ebadf-retry-class.md` — the eval that scores the task.

## Design Decisions
The seam is a package variable holding `(*exec.Cmd).CombinedOutput` rather than an injected runner on `Repo`, because the fixture constructors take only a `testing.TB` and adding a field to every caller's path would widen the API for test-only fault injection. The classification uses `errors.Is` on the wrapped chain, never message text, so an error that only reads like "bad file descriptor" is not retried.

## Verification
`go test -count=1 ./internal/gittest` passes, including the exec-fault tests; `go test -count=1 -tags acs ./acs/cycle1834` passes 7/7; ship's five `TestCaptureWithEBADFRetry_*` integration tests pass on the shared rule; the 22 packages that import gittest pass except one ship test that fails identically at the base commit.

## Compatibility
`Repo.Git`, `Fixture`, `Bare` and `Clone` keep their signatures and their failure message format. A successful git run executes once, as before.

## Limitations
Only one retry is made; a fault that persists across two execs still fails the test. Raw `exec.Command` git calls outside gittest and ship's helpers are not covered.
