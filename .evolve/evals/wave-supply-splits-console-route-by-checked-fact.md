---
score_cap:
  - criterion: "a console route with only unprotected files counts as console_route_no_fact"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2457_001 ./acs/cycle2457"
  - criterion: "a console route with a protected declared file counts as console_route_fact"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2457_002 ./acs/cycle2457"
  - criterion: "evolve wave supply --json carries both counts and their sum is the old console_route count"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2457_003 ./acs/cycle2457"
---

# Eval: wave supply splits the console route by the checked fact

> Pins plan K2 of docs/plans/console-build-or-route-2026-11.md. The census counted every
> route:console item as console_route, so the console could not tell a route that a protected
> file explains from a route with no reason. Cycle 2457 splits it into console_route_fact and
> console_route_no_fact.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-fact | unprotected-file route is no_fact | 6/10 | `go test -tags acs -run TestC2457_001 ./acs/cycle2457` |
| fact | protected-file route is fact | 6/10 | `go test -tags acs -run TestC2457_002 ./acs/cycle2457` |
| json-sum | both counts in JSON, sum = old count | 7/10 | `go test -tags acs -run TestC2457_003 ./acs/cycle2457` |
