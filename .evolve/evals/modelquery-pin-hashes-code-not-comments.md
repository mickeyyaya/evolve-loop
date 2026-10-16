---
score_cap:
  - criterion: "A comment-only edit to any decision-surface file leaves the modelquery decision-surface pin unchanged"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1882_001_CommentOnlyEditLeavesPinUnchanged ./acs/cycle1882"
  - criterion: "A code edit to any decision-surface file still breaks the pin"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1882_002_CodeEditChangesPin ./acs/cycle1882"
  - criterion: "classifier.go, latest.go, complete.go, lineage.go and newestwins.go carry no comments beyond a package doc"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1882_004_DecisionSurfaceFilesCarryNoComments ./acs/cycle1882"
  - criterion: "The comment-reading TestOllamaListMetadataExceptionDocumented is retired and TestOllamaListerReachesNoModel stays"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1882_005_CommentReadingOllamaTestRetired ./acs/cycle1882"
  - criterion: "TestOllamaLister_NilRunDefaultsToExecRunner fails when the lister drops the listing on a successful ollama run"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1882_00(6|7)_' ./acs/cycle1882"
  - criterion: "The model catalog design doc no longer lists agy under the tmux recipe picker"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run TestC1882_008_CatalogDocDropsAgyFromRecipePicker ./acs/cycle1882"
  - criterion: "The modelquery package tests pass"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/modelquery"
---

# Eval: modelquery's decision-surface pin hashes code, not comments

> Pins the fix for inbox item modelquery-pin-hashes-code-not-comments (comment round 12, PR #749).
> TestDecisionVersion_PinnedToAlgorithmSurface hashed raw bytes, so any comment edit moved the pin and
> five files could not shed their comments. The pin must now track the code shape only, while a code
> edit still breaks it. Source incident: round-12 deferral, picked up in cycle 1882.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| comment-insensitive pin | comment-only edit keeps the pin | 8/10 | `TestC1882_001` |
| code-sensitive pin (negative) | code edit breaks the pin | 8/10 | `TestC1882_002` |
| zero comments | five files carry no comments | 6/10 | `TestC1882_004` |
| retired source-reading test | no test reads ollama.go text | 5/10 | `TestC1882_005` |
| assertive nil-run test | mutant lister is caught; real lister passes | 6/10 | `TestC1882_006`, `TestC1882_007` |
| doc accuracy | agy not under recipe picker | 3/10 | `TestC1882_008` |
| package green | modelquery suite passes | 9/10 | `go test ./internal/modelquery` |
