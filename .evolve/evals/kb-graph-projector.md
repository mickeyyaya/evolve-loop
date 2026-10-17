---
score_cap:
  - criterion: "Neighbors links cycles through a shared fingerprint within two hops"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1936_101' ./acs/cycle1936"
  - criterion: "hops are capped at 2 and nodes at 40"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1936_102|TestC1936_103' ./acs/cycle1936"
  - criterion: "absent edges are reported as absent and never synthesized"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1936_104|TestC1936_108' ./acs/cycle1936"
  - criterion: "evolve kb graph recurrences surfaces the second instance"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1936_109' ./acs/cycle1936"
---

# Eval: kb graph projector slice 1

> Pins slice 1 of the kb-graph-projector inbox item: a read-only, deterministic, in-memory projector over knowledge-base/cycles and the inbox, with the evolve kb graph verbs. Slices 2 and 3 stay deferred (cycle 1936).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| traversal | shared fingerprint links cycles | 5/10 | `go test -run TestC1936_101` |
| caps | hops<=2, nodes<=40 | 5/10 | `go test -run TestC1936_102` |
| absent | absent edges stay ABSENT | 6/10 | `go test -run TestC1936_104` |
| recurrence | second instance surfaces | 6/10 | `go test -run TestC1936_109` |
