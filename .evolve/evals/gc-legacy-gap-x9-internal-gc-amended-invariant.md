---
score_cap:
  - criterion: "internal-gc.md has an Amended by line that links the plan and names its decision ids"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2267_001|TestC2267_004' ./acs/cycle2267"
  - criterion: "The docs check rejects a note without the Amended by line, a note with the old wording and an empty note"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2267_002' ./acs/cycle2267"
  - criterion: "The amended passages of the note pass the STE lint"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2267_003' ./acs/cycle2267"
---

# Eval: gc X9 internal-gc.md amended invariants

> Pins component X9 of docs/plans/gc-closes-the-legacy-gap-2026-11.md (D9). The note gets an Amended by line that links the plan. The branch invariant says a branch is deleted only after D2, when it is ended (D6) or landed (D4). The hook invariant names gc.wave_end.scope at full under D2 and D7. Source: cycle 2267.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| amended-line | the Amended by line links the plan and names D2, D4, D6, D7, D9 | 3/10 | `TestC2267_001`, `TestC2267_004` |
| check-rejects | each missing amendment fails the check | 5/10 | `TestC2267_002` |
| ste-clean | the amended passages pass the STE lint | 6/10 | `TestC2267_003` |
