---
score_cap:
  - criterion: "A real scout or triage failure before build charges the item the lineage record pins"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2668_001' ./acs/cycle2668"
  - criterion: "An item that cannot be parked at its retry ceiling keeps its continuation"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2668_002' ./acs/cycle2668"
  - criterion: "A host fault charges no item, and a real fault of the same shape still does"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2668_003' ./acs/cycle2668"
---

# Eval: The breaker charges each failure before build to the pinned item

> Pins plan queue-authority-from-host-facts B1 (D10, D11, D13). A scout or triage failure before build
> raised no `failure_count`, so each wave claimed the same item again (cycles 2133 and 2151). The predicates
> drive `cycleoutcome.FailureInputsFor` and `cycleoutcome.ApplyFailure`, the pair that
> `go/cmd/evolve/cmd_cycle.go` calls on a failed cycle.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| charge-prebuild | Pre-build failure charges the pinned item, not a bystander, and a failure after build stays uncharged | 4/10 | `go test -tags acs -run TestC2668_001 ./acs/cycle2668` |
| keep-continuation | A park that fails leaves the continuation on the item; a park that succeeds retires it | 5/10 | `go test -tags acs -run TestC2668_002 ./acs/cycle2668` |
| host-fault-exempt | Host-fault code and infrastructure scout charge nothing; a real code charges | 5/10 | `go test -tags acs -run TestC2668_003 ./acs/cycle2668` |
