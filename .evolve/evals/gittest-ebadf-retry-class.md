---
score_cap:
  - criterion: "A gittest fixture whose first git exec fails with EBADF or a closed pipe passes after exactly one retry on a fresh exec.Cmd, and one whose exec fails twice still fails the test naming the git command and the EBADF"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^(TestFixture_RetriesAGitInitWhoseFirstExecFailsTransiently|TestFixture_PersistentEBADFFailsTheTestNamingTheError)$' ./internal/gittest/"
  - criterion: "gittest.CaptureWithEBADFRetry absorbs one transient EBADF/closed pipe and returns a persistent one after exactly one retry"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^(TestC1834_001_CaptureWithEBADFRetryAbsorbsOneTransientEBADFOrClosedPipe|TestC1834_002_CaptureWithEBADFRetryReturnsAPersistentEBADFAfterExactlyOneRetry)$' ./acs/cycle1834/"
  - criterion: "A persistent non-EBADF error from git (exit status, EMFILE, EPIPE, os.ErrClosed, missing binary, EBADF lookalike text) is never retried by Repo.Git or CaptureWithEBADFRetry"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRepoGit_NeverRetriesANonEBADFFailure$' ./internal/gittest/ && go test -count=1 -tags acs -run '^(TestC1834_003_CaptureWithEBADFRetryNeverRetriesANonEBADFError|TestC1834_004_IsEBADFLikeClassifiesOnlyEBADFAndClosedPipe)$' ./acs/cycle1834/"
  - criterion: "ship's captureWithEBADFRetry and gittest share one retry rule: ship only delegates to gittest.CaptureWithEBADFRetry, classifies no EBADF itself, and its five retry-contract tests pass on the shared rule"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1834_006_ShipsEBADFRetryIsGittestsOneRuleWithNoSecondCopy$' ./acs/cycle1834/"
  - criterion: "gittest stays apicover-enforced: the new CaptureWithEBADFRetry and IsEBADFLike exports are named and covered by gittest's own tests"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1834_007_GittestApicoverGateNamesAndCoversTheSharedRetryExports$' ./acs/cycle1834/"
---

# Eval: Give the shared gittest git runner the EBADF retry that one ship test path already has

> This eval pins the cycle-1834 fix for the second sighting of the real-git EBADF race.
> In PR #753 (CI run 36719406746), the ship test
> `TestShipFromWorktree_DiffCachedQuietFails_Errors` failed on ubuntu-latest with
> `gittest: git init -q -b main ...: fork/exec /usr/bin/git: bad file descriptor`.
> The PR did not touch ship or gittest. The first sighting was on macOS in cycle 249:
> see `macos-ebadf-test-hardening.md`. That fix retried only ship's own git runner
> (`captureWithEBADFRetry`). The shared fixture package `internal/gittest`, which
> every package's real-git tests use, had no retry. This eval holds three things:
> the one-retry rule lives in gittest's command runner (`Repo.Git`, and through it
> the fixture's init); ship delegates to that one rule; and no non-EBADF failure is
> ever retried, so the retry never hides a real git failure.
> Source: inbox `gittest-ebadf-retry-class` (console, 2026-09-30).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fixture-retry | A forced first-exec EBADF in `Fixture` is retried once on a fresh `exec.Cmd`. A persistent one fails the test, naming `git init` and the EBADF. | 8/10 | gittest `TestFixture_*` exec-fault tests |
| shared-rule-semantics | `CaptureWithEBADFRetry` absorbs one transient error and returns a persistent one after exactly 2 runs. | 7/10 | ACS `TestC1834_001`, `TestC1834_002` |
| no-masking | Non-EBADF errors run once and pass through unchanged. A lookalike error message is not classified as EBADF. | 8/10 | gittest `TestRepoGit_NeverRetriesANonEBADFFailure`, ACS `TestC1834_003`, `TestC1834_004` |
| one-rule | ship's helper delegates to gittest and classifies no EBADF itself. Its 5 retry tests still pass. | 7/10 | ACS `TestC1834_006` |
| apicover-graduation | gittest's own tests name and cover the new exports. | 5/10 | ACS `TestC1834_007` |
