---
score_cap:
  - criterion: "DispatchModelChain returns the live claude deep chain fable, opus, sonnet and nil for a non-live entry"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestDispatchModelChain_' ./internal/modelcatalog"
  - criterion: "A wall on the first model of a tier succeeds on the second model without the quota exit"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestLaunch_AQuotaWallOnTheFirstModelOfATierSucceedsOnTheNextModel|TestLaunch_TheAuditorSucceedsOnOpusWhenFableHitsItsQuotaWall' ./internal/bridge"
  - criterion: "The quota exit is terminal only when every model of the chain walls"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestLaunch_OnlyAWallOfTheWholeChainReturnsTheQuotaExit|TestLaunch_TheAuditorReturnsTheQuotaExitOnlyWhenFableOpusAndSonnetAllWall' ./internal/bridge"
  - criterion: "A claude-floored auditor keeps the fable alias in its chain and steps to it from opus"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestLaunch_TheAuditorKeepsFableWhenTheCatalogPrimaryIsOpus|TestLaunch_TheFamilyFloorRecognisesEveryClaudeAliasOfTheLiveChain' ./internal/bridge"
---

# Eval: Within-tier model failover for the deep auditor

> Pins the model axis of the fallback policy (model inside tier inside CLI). Source incident: cycles 908-915 stopped at the Fable 5 quota wall while Opus had budget. Cycle 2128 found the engine loop in place behind bridge.model_failover, and the claude family floor dropping the fable alias from a chain.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| chain-order | live claude deep chain is fable, opus, sonnet | 6/10 | `go test -run TestDispatchModelChain_ ./internal/modelcatalog` |
| model-step | wall on model 1 succeeds on model 2 | 4/10 | `go test -run TestLaunch_TheAuditorSucceedsOnOpusWhenFableHitsItsQuotaWall ./internal/bridge` |
| whole-chain-wall | exit 85 only after every model walls | 5/10 | `go test -run TestLaunch_TheAuditorReturnsTheQuotaExitOnlyWhenFableOpusAndSonnetAllWall ./internal/bridge` |
| floor-alias | floored auditor keeps fable | 4/10 | `go test -run TestLaunch_TheAuditorKeepsFableWhenTheCatalogPrimaryIsOpus ./internal/bridge` |
