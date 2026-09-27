---
score_cap:
  - criterion: "AC1 `commentaudit comments -base <ref>`, run through the real binary against a real git diff, lists every added whole-line comment as `path: text` — plain, `See ADR-NNNN.` pointer, exported doc, narrative, one-line block, test-file and new-file comments — never a comment already at base, and exits 1"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1723_001_CommentsListsEveryAddedCommentLine$' ./acs/cycle1723/"
  - criterion: "AC1 machine-read directives (//go:generate, //go:noinline, //nolint, // Deprecated:, //apicover:ignore, // minimal:, // acs-predicate:, IPC-protocol-allowed, // Output:) are never listed; a directive-only diff exits 0"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1723_002_CommentsExcludesMachineReadDirectives$' ./acs/cycle1723/"
  - criterion: "AC1 only a file new at base has its package doc exempt; the new file's other comments are listed, and a package doc added or rewritten in a file that existed at base is listed"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1723_003_CommentsExemptsOnlyANewFilesPackageDoc$' ./acs/cycle1723/"
  - criterion: "AC1 a comment moved within its file is not added, a second copy of a base comment is added exactly once (multiset), and a removed comment is not added"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1723_004_CommentsDoesNotCountAMovedLineAsAdded$' ./acs/cycle1723/"
  - criterion: "AC1 exit codes: 0 with `no comments added in N changed Go file(s)` when nothing is listed, 2 with a usage line naming `comments` when -base is missing or empty, 1 when the base ref is unreadable"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1723_005_CommentsExitCodesAndUsage$' ./acs/cycle1723/"
  - criterion: "AC1 `[dir ...]` scopes the listing like check/verify, and a scope matching no changed Go file fails"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1723_006_CommentsScopesToDirs$' ./acs/cycle1723/"
  - criterion: "Regression guard: `check` keeps its narrative-only contract over the same diff"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1723_007_CheckKeepsItsNarrativeOnlyContract$' ./acs/cycle1723/"
  - criterion: "internal/commentaudit and cmd/commentaudit stay vet-, gofmt- and test-clean, carry durable unit tests for the subcommand (`go test -run Comments`), and every export of the apicover-enforced package stays named and executed"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run Comments ./internal/commentaudit/ | grep -q -- '--- PASS: '"
---

# Eval: commentaudit lists every comment line a diff adds

> Pins `commentaudit comments -base <ref> [dir ...]`. It lists every whole-line
> comment the diff against a base adds. It excludes the machine-read directives
> that `Directives` already knows and the package doc of a file that is new at
> base. A comment line that moved within its file is not counted. The command
> exits 1 when it lists any line. Before this, `commentaudit check` flagged only
> history-carrying lines, so reviewers had to count every other added comment by
> reading the diff. Source: inbox item `commentaudit-reports-every-added-comment`
> (cycle 1723). The triage `top_n` card is `commentaudit-comments-subcommand`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| every-comment | every added whole-line comment is listed, base comments are not | 8/10 | `TestC1723_001` |
| directives-excluded | machine-read directives never listed | 7/10 | `TestC1723_002` |
| new-package-doc-only | package-doc exemption only for a file new at base | 7/10 | `TestC1723_003` |
| moved-not-added | move-aware multiset diff | 7/10 | `TestC1723_004` |
| exit-codes | 0 / 1 / 2 contract and the summary line | 6/10 | `TestC1723_005` |
| dir-scope | `[dir ...]` scoping like check/verify | 5/10 | `TestC1723_006` |
| check-unchanged | `check` stays narrative-only | 5/10 | `TestC1723_007` |
| durable-tests | package unit tests named `Comments` pass | 6/10 | `go test -run Comments -v ./internal/commentaudit/` |
