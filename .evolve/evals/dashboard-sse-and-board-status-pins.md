---
score_cap:
  - criterion: "The three pinning tests exist and pass at HEAD"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestServer_SSEStreamOutlivesAnyWriteDeadline|TestServer_SSEFirstSnapshotIDIsNeverRepeated|TestHandleCycle_BoardLaneStatusWins)$' ./internal/dashboard"
  - criterion: "Each pinning test fails when its rule is removed (mutation kill via go test -overlay)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1849_00[234]' ./acs/cycle1849"
  - criterion: "The two kept why-comments in server.go and sse.go are gone"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1849_005 ./acs/cycle1849"
  - criterion: "The run-workspace cycle cap (design note: newest workspaces kept, running lanes exempt) is pinned by a cap-named test"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1849_006 ./acs/cycle1849"
---

# Eval: dashboard — pin the SSE write-timeout and subscribe-order rules, the board lane-status overlay, and the cycle cap

> Comment round 12 (PR #749, docs/reports/comment-round-12-findings-2026-09-30.md) found two kept
> whys in the dashboard with no pinning test, a board lane-status overlay in handleCycle that
> survives deletion in every test, and a selectCycles cap whose truncation of run workspaces was
> unpinned. Cycle 1849 adds tests whose value is proven by mutation: each rule is removed through a
> `go test -overlay` copy and the named test must fail. The cap keeps its documented behaviour
> (docs/architecture/packages/internal-dashboard.md: newest workspaces kept, running lanes exempt).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| presence | three named tests pass | 6/10 | `go test -run '^(...)$' ./internal/dashboard` |
| mutation-kill | each test fails without its rule | 8/10 | `TestC1849_002..004` |
| comment removal | no prose comment in server.go, sse.go | 5/10 | `TestC1849_005` |
| cap pin | removing the workspace truncation fails a cap test | 6/10 | `TestC1849_006` |
