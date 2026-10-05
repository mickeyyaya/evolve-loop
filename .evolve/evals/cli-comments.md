---
score_cap:
  - criterion: "evolve comments verify on a comment-only diff reports equivalence exactly as commentaudit verify does, and rejects a code edit the same way"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_020' ./acs/cycle1792"
  - criterion: "a relative directory argument works for verify: cwd-relative, repo-root-relative from a subdirectory, and dot, matching commentaudit"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_021' ./acs/cycle1792"
  - criterion: "both entry points run commentaudit.Main: identical stdout, stderr and exit code for every subcommand, with commentaudit's usage exit 2"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_022' ./acs/cycle1792"
  - criterion: "docs/conventions/code-comments.md documents a runnable evolve comments verify and runtime-reference.md documents evolve comments"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_023' ./acs/cycle1792"
---

# Eval: evolve comments

> Pins `evolve comments <rank|check|comments|verify|history|strip> ...`, cycle 1792 (inbox item cli-comments). The comment campaign's proof tools ran only as `go run ./cmd/commentaudit`. The predicates build both binaries and require byte-identical results across a subcommand matrix on a git fixture, so the verb must delegate to `commentaudit.Main` unchanged (the api-contract keeps `cmd/commentaudit` as a thin wrapper and adds no argument rewriting, since `scopeDirs` already resolves relative directories).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| verify-parity | comment-only verify and code-edit rejection match commentaudit | 8/10 | `go test -tags acs -run 'TestC1792_020' ./acs/cycle1792` |
| relative-dirs | cwd-relative, root-relative and dot operands | 7/10 | `go test -tags acs -run 'TestC1792_021' ./acs/cycle1792` |
| one-main | every subcommand identical, usage exit 2 | 7/10 | `go test -tags acs -run 'TestC1792_022' ./acs/cycle1792` |
| comments-doc | the convention documents the verb | 4/10 | `go test -tags acs -run 'TestC1792_023' ./acs/cycle1792` |
