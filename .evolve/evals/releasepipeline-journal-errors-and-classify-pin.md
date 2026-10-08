---
score_cap:
  - criterion: "A journal write failure after init makes Run return an error that names the journal location, at every stage that writes the journal (pre-publish, ship, marketplace-poll, release-verify), and a pre-publish failure stops the run before Ship"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -v -run '^TestRun_JournalWriteError$' ./internal/releasepipeline | grep -q -- '--- PASS: TestRun_JournalWriteError '"
  - criterion: "TestRun_ReleaseClassComputedBeforeShipCommits pins that Ship receives release notes classified against the pre-ship tree (config-release), not the tree after Ship commits the rebuilt go/evolve"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -run '^TestRun_ReleaseClassComputedBeforeShipCommits$' ./internal/releasepipeline | grep -q -- '--- PASS: TestRun_ReleaseClassComputedBeforeShipCommits '"
  - criterion: "The why comment above the release-class computation in release_run.go is gone and no comment line is added to release_run.go or releasepipeline.go"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1832_005_' ./acs/cycle1832 | grep -q -- '--- PASS: TestC1832_005_'"
  - criterion: "initJournal takes only (Options, time.Time): the never-read fromTag parameter is dropped and the journal init tests still pass"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1832_002_' ./acs/cycle1832 | grep -q -- '--- PASS: TestC1832_002_'"
  - criterion: "resolveInitCommit returns an error on empty or whitespace-only git output and has no unreachable branch (100% statement coverage under its own tests)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1832_003_' ./acs/cycle1832 | grep -q -- '--- PASS: TestC1832_003_'"
---

# Eval: releasepipeline surfaces journal write errors and pins classify-before-ship

> Pins inbox item `releasepipeline-journal-errors-and-classify-pin` (filed from the PR #749 comment round, built in cycle 1832).
> `appendStep` and `setJournalField` dropped the error from `writeJournal`. Thus, a write that failed after
> `initJournal` left rollback's journal incomplete, and `Run` still reported success. The release-class
> computation in `release_run.go` had only a comment to keep it before `Ship`. If `Ship` commits the rebuilt
> `go/evolve` first, every release diffs as a binary-release. This eval makes a test hold that order, so that
> the comment can go. It also pins the two hygiene fixes from the same item: the unused `fromTag` parameter of
> `initJournal`, and the dead `len(lines) == 0` branch of `resolveInitCommit`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| journal-write-surfaces | A journal write failure after init returns an error from Run at each journal-writing stage | 8/10 | `go test -run '^TestRun_JournalWriteError$' ./internal/releasepipeline` |
| classify-before-ship | Ship gets the pre-ship (config-release) class, not the post-commit one | 7/10 | `go test -run '^TestRun_ReleaseClassComputedBeforeShipCommits$' ./internal/releasepipeline` |
| why-comment-gone | release_run.go has no comment; no comment added to either file | 6/10 | `go test -tags acs -run '^TestC1832_005_' ./acs/cycle1832` |
| unused-param-dropped | initJournal(Options, time.Time) | 5/10 | `go test -tags acs -run '^TestC1832_002_' ./acs/cycle1832` |
| dead-branch-dropped | resolveInitCommit errors on empty output; 100% statement coverage | 5/10 | `go test -tags acs -run '^TestC1832_003_' ./acs/cycle1832` |

## Negative and Edge Cases

- Negative: the journal directory is replaced by a file in the middle of a run (a write fails with ENOTDIR, also as root). `Run` must return a non-nil error. A pre-publish failure must wrap `ErrPrePublishFailed`, and `Ship` must not run.
- Control: the same harness with a writable journal returns nil. Thus, the negative rows cannot pass because of an unrelated failure.
- Edge: fake `git` output `""` and `"\n  \n"` must give an error. `"abc1234\n"` and `"abc1234\ndef5678\n"` must give `abc1234`.
- Mutants: two mutants were run through `go test -overlay`. In the first, `Ship` receives the notes computed after it ran. In the second, the classification is always binary. `TestRun_ReleaseClassComputedBeforeShipCommits` failed for both.
