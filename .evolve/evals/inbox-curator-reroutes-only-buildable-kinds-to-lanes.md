---
score_cap:
  - criterion: "A research, decision or task item routed console-manual gets no lane reroute proposal from evolve inbox curate"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2404_001 ./acs/cycle2404"
  - criterion: "An item that names no file path gets no lane reroute proposal from evolve inbox curate"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2404_002 ./acs/cycle2404"
---

# Eval: the inbox curator reroutes only buildable kinds that name files to lanes

> Pins the two acceptance criteria of the inbox item. The curator proposed a lane reroute for any console-manual item that no protected surface explained, including research, decision and task items and items with no file path, which a lane cannot build. Each predicate runs the real `evolve inbox curate --json` verb and also requires a buildable control item to keep its lane reroute, so a curator that proposes nothing cannot pass. Source incident: cycle 2387 audit; cycle 2404 re-implements it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| unbuildable-kind | research, decision, task get no lane reroute | 6/10 | `go test -tags acs -run TestC2404_001 ./acs/cycle2404` |
| fileless | an item with no named file gets no lane reroute | 6/10 | `go test -tags acs -run TestC2404_002 ./acs/cycle2404` |
