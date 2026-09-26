---
score_cap:
  - criterion: "go test -count=1 -tags acs ./acs/cycle50 is green, with TestC50B_002 and TestC50B_005 actually passing (not deleted or skipped)"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1702_001_Cycle50SuiteGreen$' ./acs/cycle1702/"
  - criterion: "Every predicate that read a file renamed out of go/cmd/evolve now resolves that file's current location under go/internal/cli/{opscmd,guardcmd,phasecmd}"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1702_002_RepointedPredicatesResolveTheRenamedFile$' ./acs/cycle1702/"
  - criterion: "The repointed sibling predicates in cycles 11, 31, 47 and 48 pass through the real ACS runner"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1702_003_RepointedSiblingPredicatesPass$' ./acs/cycle1702/"
  - criterion: "acsassert FileExists/FileContains/FileNotContains/FileMatchesRegex fail a nonexistent path with a message naming the moved-file possibility and the path"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1702_004_NotExistReadHintsAtRelocation$' ./acs/cycle1702/"
  - criterion: "The moved-file hint is absent for content mismatches and non-ENOENT read errors, and satisfied assertions log nothing"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1702_005_HintOnlyForNotExist$' ./acs/cycle1702/"
  - criterion: "A sweep of every go/acs cycle and regression predicate file finds no repo-root Go-source path that no longer exists"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1702_00[67]_' ./acs/cycle1702/"
  - criterion: "Every exported acsassert path reader that can report a failure (a TB parameter or an error result), including CountInGoFunc and JSONFieldEquals, fails a missing path naming the path and the moved-file possibility, and an error result still satisfies errors.Is(err, fs.ErrNotExist)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1702_008_EveryMessageCarryingReaderHintsAtRelocation$' ./acs/cycle1702/"
  - criterion: "CountInGoFunc and JSONFieldEquals keep a plain failure for an existing path (missing function, unparsable source, invalid JSON, absent key, mismatch, directory) and succeed silently when satisfied"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1702_009_ErrorAndJSONReadersHintOnlyForNotExist$' ./acs/cycle1702/"
  - criterion: "The value-only acsassert path readers, which cannot carry a message without an API change, are named in the acceptance of an inbox item that declares go/pkg/acsassert"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1702_010_FollowUpInboxItemCoversTheSilentReaders$' ./acs/cycle1702/"
---

# Eval: ACS predicates resolve moved source files and say so when they cannot

> Pins the fix for inbox item `acs-cycle50-predicates-point-at-a-moved-file`
> (cycle 1702). The 2026-06-21 `cmd/evolve` decomposition (`8d3e4548`,
> `e5fc3bb1`, `a3f483a8`) renamed handler files into
> `go/internal/cli/{opscmd,guardcmd,phasecmd}/`, but 11 `filepath.Join(root, …)`
> targets across `go/acs/cycle{11,31,47,48,50}/predicates_test.go` kept the old
> `go/cmd/evolve/cmd_*.go` paths. The migrations those predicates guard were
> already complete, yet every one reported a false RED with a bare
> `no such file or directory`. The inbox acceptance asks for three things:
> cycle 50 green, a "may have moved" message from any source-path read, and a
> sweep proving no other predicate points at a missing path.
>
> Cycle 1702's round-2 audit FAILed (H1) because the first contract covered
> only the four TB-taking helpers. `CountInGoFunc` (17 predicate files) and
> `JSONFieldEquals` still returned a bare `no such file or directory`. The
> value-only readers `FileContainsAny`, `CountOccurrencesAny` and
> `LineContainsAll` (49 files) cannot carry any message. The last three rows
> enumerate the whole reader population from the package source, so a reader
> added later without the hint fails the cap. They also require the
> value-only gap to be queued as an inbox item rather than dropped.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cycle50-green | cycle 50 suite exits 0 with both preflight predicates PASSing | 9/10 | `TestC1702_001_Cycle50SuiteGreen` |
| exact-repoint | each stale join now names the renamed file, not some other file | 8/10 | `TestC1702_002_RepointedPredicatesResolveTheRenamedFile` |
| sibling-green | cycle 11/31/47/48 repointed predicates PASS via `go test -tags acs` | 8/10 | `TestC1702_003_RepointedSiblingPredicatesPass` |
| moved-hint | not-exist read names the moved-file possibility | 7/10 | `TestC1702_004_NotExistReadHintsAtRelocation` |
| hint-negative | no hint on mismatch / directory read; silent on success | 6/10 | `TestC1702_005_HintOnlyForNotExist` |
| sweep | zero missing repo-root `.go` join targets; sweep self-test detects a stale join | 8/10 | `TestC1702_006_…`, `TestC1702_007_…` |
| reader-population | every message-carrying path reader (enumerated from source) hints; `errors.Is` kept | 8/10 | `TestC1702_008_EveryMessageCarryingReaderHintsAtRelocation` |
| reader-hint-negative | no hint from CountInGoFunc/JSONFieldEquals on an existing path; silent on success | 6/10 | `TestC1702_009_ErrorAndJSONReadersHintOnlyForNotExist` |
| value-reader-follow-up | value-only readers named in a queued `go/pkg/acsassert` inbox item | 5/10 | `TestC1702_010_FollowUpInboxItemCoversTheSilentReaders` |
