---
score_cap:
  - criterion: "One --version probe per CLI binary per looppreflight Run"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestRun_VersionInventoryProbedOncePerRun|TestRun_DefaultInventoryProbesEachBinaryOnce' ./internal/looppreflight"
  - criterion: "Version cache write survives a occupied fixed temp name and concurrent writers"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestSaveVersionCache_|TestRun_CacheWrittenDespiteStaticTempOccupied' ./internal/looppreflight"
  - criterion: "goodPipelineOptions stubs every process seam so Run execs no real subprocess"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestGoodPipelineOptions_StubsEveryProcessSeam|TestRun_GoodPipelineOptionsExecNoVersionProbe' ./internal/looppreflight"
  - criterion: "cycle270 ACS predicates pass against current looppreflight code"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs ./acs/cycle270"
---

# Eval: looppreflight version inventory cache

> Pins the single-capture and atomic-write contract of the looppreflight version inventory. Run captured the inventory twice (drift check and Result.CLIVersions) and saveVersionCache wrote through a fixed `.tmp` name. Source incident: inbox item looppreflight-version-cache, cycle 1854.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| single-capture | one probe per binary per Run | 6/10 | `go test -run TestRun_VersionInventoryProbedOncePerRun` |
| atomic-cache | no fixed-temp collision | 7/10 | `go test -run TestSaveVersionCache_` |
| hermetic-tests | no real exec in unit tests | 6/10 | `go test -run TestGoodPipelineOptions_StubsEveryProcessSeam` |
| cycle270-green | stale predicate repointed | 7/10 | `go test -tags acs ./acs/cycle270` |
