---
score_cap:
  - criterion: "a call through the route writes backend=gotypes for callers and backend=trail for find"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2297_001|TestC2297_002' ./acs/cycle2297"
  - criterion: "evolve code usage groups the records by backend"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2297_003|TestC2297_004' ./acs/cycle2297"
  - criterion: "an old record with no backend field still reads and shows as unknown"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2297_005|TestC2297_006' ./acs/cycle2297"
---

# Eval: the code index usage log names the backend (code index G5)

> Pins plan code-index-backends-2026-10, component G5. Each usage record carries the backend that answered the call, and `evolve code usage` groups by it. The decision to keep or replace a backend rests on this log (ADR-0138). Source incident: cycle 2282 built the field and failed audit on an unrelated suite abort, so the work continues in cycle 2297.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| route | callers logs gotypes, find logs trail, and the name follows the backend, not a constant | 7/10 | `TestC2297_001`, `TestC2297_002` |
| grouping | each backend has its own row in the table and in JSON | 7/10 | `TestC2297_003`, `TestC2297_004` |
| legacy | an old line reads, and an empty backend shows as unknown | 6/10 | `TestC2297_005`, `TestC2297_006` |
