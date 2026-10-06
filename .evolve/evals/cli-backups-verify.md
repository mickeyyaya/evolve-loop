---
score_cap:
  - criterion: "backups verify exits 0 only when every bundle head and patch exists elsewhere"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1807_00[1-5]' ./acs/cycle1807"
  - criterion: "backups verify refuses a head only in the backup and names its SHA"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1807_002 ./acs/cycle1807"
  - criterion: "backups verify keeps I/O errors (exit 2) distinct from refusals (exit 1)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1807_006 ./acs/cycle1807"
  - criterion: "backups verify judges patches against origin/main, never the checkout or the index"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1807_(016|017|018|019|020)' ./acs/cycle1807"
---

# Eval: evolve backups verify

> Pins the read-only `backups verify` verb: heads classified IN_MAIN / OTHER_BRANCH / ONLY_IN_BACKUP, patches APPLIED / UNAPPLIED, exit 0/1/2 semantics, and the gitexec helpers behind it. Source: cycle 1807 inbox item cli-backups-verify; audit round 1 M1 found patches checked against the working tree, so a change present only in the checkout was reported APPLIED (safe to delete) while origin/main lacked it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| safe-path | all items elsewhere, dir untouched | 6/10 | `go test -tags acs -run 'TestC1807_00[1-5]' ./acs/cycle1807` |
| unsafe-head | ONLY_IN_BACKUP refuses, SHA named | 7/10 | `go test -tags acs -run TestC1807_002 ./acs/cycle1807` |
| exit-2 | I/O errors are not refusals | 6/10 | `go test -tags acs -run TestC1807_006 ./acs/cycle1807` |
| origin-main-patches | APPLIED means origin/main contains it | 8/10 | `go test -tags acs -run 'TestC1807_(016|017|018|019|020)' ./acs/cycle1807` |
