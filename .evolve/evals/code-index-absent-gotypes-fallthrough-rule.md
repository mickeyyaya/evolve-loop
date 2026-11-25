---
score_cap:
  - criterion: "The policy key code_index.on_absent defaults to refuse, accepts next, and rejects any other word"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestCodeIndexOnAbsent_' ./internal/policy/"
  - criterion: "With the fall-through option an absent backend routes to the next capable backend and served names it; not-found and other errors never fall through"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestBackendsOnAbsent_' ./internal/codeindex/"
  - criterion: "The evolve code command reaches the rule from the policy file, exits 2 in refuse mode, and still exits 21 for a stale index under stale=refuse"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCodeOnAbsentWiring_' ./cmd/evolve/"
  - criterion: "The spec and the plan name on_absent and the spec no longer says there is no failover"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC2637_005' ./acs/cycle2637/"
---

# Eval: the rule for an absent gotypes index has one home

> Pins the rule of research dossier D1 and open question 6 as the policy key
> `code_index.on_absent` (`refuse` by default, `next`). The spec said "no failover" and
> the plan said "fall to trail"; the key is the one place where the rule lives, and the
> docs describe it. Source incident: cycle 2637 task `code-index-absent-gotypes-fallthrough-rule`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| policy-key | default refuse, `next` accepted, other words rejected | 5/10 | `go test -run TestCodeIndexOnAbsent_ ./internal/policy/` |
| route-rule | absent falls to the next capable backend; not-found and other errors do not | 5/10 | `go test -run TestBackendsOnAbsent_ ./internal/codeindex/` |
| caller-proof | the command reads the key from policy.json and logs the answering backend | 6/10 | `go test -run TestCodeOnAbsentWiring_ ./cmd/evolve/` |
| docs-agree | spec and plan agree on the rule | 7/10 | `go test -tags acs -run TestC2637_005 ./acs/cycle2637/` |
