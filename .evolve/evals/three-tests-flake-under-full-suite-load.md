---
score_cap:
  - criterion: "TestSyncMain_TheLoopsInboxStampsNeverBlockTheSync passes 50 runs"
    max_if_missing: 6
    evidence: "cd go && go test -count=50 -run '^TestSyncMain_TheLoopsInboxStampsNeverBlockTheSync$' ./cmd/evolve"
  - criterion: "TestChannelE2E_RealFixtures_ClaudeSpan passes 50 runs"
    max_if_missing: 6
    evidence: "cd go && go test -count=50 -run '^TestChannelE2E_RealFixtures_ClaudeSpan$' ./internal/bridge"
  - criterion: "every git repo of the SyncMain fixtures disables git maintenance"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestSyncMainFixture_' ./cmd/evolve"
---

# Eval: two tests that fail once under full-suite load

> Pins the stability of `TestChannelE2E_RealFixtures_ClaudeSpan` and `TestSyncMain_TheLoopsInboxStampsNeverBlockTheSync`. Each failed once in a full `go test ./...` run on 2026-10-17 and passed alone. The SyncMain fixtures build a work repo and a console clone without `maintenance.auto=false` and `gc.auto=0` (census class F4.5). Source incident: inbox item `three-tests-flake-under-full-suite-load`, cycle 2339.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| stress-syncmain | 50 runs of the SyncMain stamps test | 6/10 | `go test -count=50 -run '^TestSyncMain_TheLoopsInboxStampsNeverBlockTheSync$' ./cmd/evolve` |
| stress-claudespan | 50 runs of the ClaudeSpan test | 6/10 | `go test -count=50 -run '^TestChannelE2E_RealFixtures_ClaudeSpan$' ./internal/bridge` |
| hermetic-git-fixture | the SyncMain fixtures disable git maintenance | 7/10 | `go test -count=1 -run '^TestSyncMainFixture_' ./cmd/evolve` |
