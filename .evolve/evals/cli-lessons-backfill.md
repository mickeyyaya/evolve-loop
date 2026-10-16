---
score_cap:
  - criterion: "evolve lessons backfill dry-run prints pattern counts and writes no ledger"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1883_001' ./acs/cycle1883"
  - criterion: "backfill --write replaces the ledger and lessons recurrence renders the same counts"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1883_002' ./acs/cycle1883"
  - criterion: "malformed lesson files are listed as skipped"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1883_003' ./acs/cycle1883"
  - criterion: "a missing lessons directory exits 2 and writes nothing"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1883_004' ./acs/cycle1883"
---

# Eval: evolve lessons backfill

> Pins the operator command that restores `.evolve/recurrence-ledger.json` from
> `.evolve/instincts/lessons/*.yaml` through `recurrence.BackfillFromLessons`.
> Before cycle 1883 the function had no CLI caller, so a damaged ledger
> undercounted history with no repair path.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dry-run | counts printed, nothing written | 7/10 | `go test -tags acs -run TestC1883_001 ./acs/cycle1883` |
| write-parity | `--write` then `lessons recurrence` shows same counts | 7/10 | `go test -tags acs -run TestC1883_002 ./acs/cycle1883` |
| skipped | malformed files listed | 6/10 | `go test -tags acs -run TestC1883_003 ./acs/cycle1883` |
| unreadable | missing directory exits 2 | 7/10 | `go test -tags acs -run TestC1883_004 ./acs/cycle1883` |
