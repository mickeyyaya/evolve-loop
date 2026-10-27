---
score_cap:
  - criterion: "runtime-reference names the code_index stale key with warn, refuse and exit 21"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2040_001 ./acs/cycle2040"
  - criterion: "runtime-reference names evolve code refresh with --json"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2040_002 ./acs/cycle2040"
  - criterion: "runtime-reference names the sync-main exit codes 22 and 23"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2040_003 ./acs/cycle2040"
---

# Eval: runtime-reference names the code index refresh surface

> Pins the three facts the code_index policy line of docs/operations/runtime-reference.md lacked: the `stale` key, the verb `evolve code refresh` and the sync-main exit codes 22 and 23. Source: inbox item runtime-reference-code-index-stale-key, cycle 2040. The spec of record is docs/architecture/code-index-cli.md section 4.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| stale-key | stale, warn, refuse, exit 21 on one line | 6/10 | `go test -run TestC2040_001` |
| refresh-verb | `evolve code refresh` with --json | 6/10 | `go test -run TestC2040_002` |
| sync-main-codes | exits 22 and 23 with sync-main | 7/10 | `go test -run TestC2040_003` |
