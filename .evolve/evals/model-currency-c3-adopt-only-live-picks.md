---
score_cap:
  - criterion: "A family whose live list failed keeps its prior live pick and emits CONFIG_CATALOG_REFRESH_DEGRADED"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCatalogDegraded_EnforceEmitsOneEventPerFamilyWhoseListFailed|TestRunStagedCatalogRefresh_EnforceKeepsThePriorLivePickOfAFamilyWhoseListFailed' ./cmd/evolve"
  - criterion: "The degraded event is a registered config.warning that the Signal Center delivers unchanged"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestRefreshDegradedEvent_' ./internal/modelcatalog"
  - criterion: "The loop boot refreshes the catalog once before the preflight and a refresher error does not abort the boot"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestRefreshCatalogAtLoopBoot_|TestRunLoopBatch_RefreshesTheCatalogOnceBeforeThePreflight' ./cmd/evolve"
  - criterion: "The TTL stops a refresh of a fresh catalog and a zero-live refresh keeps FetchedAt"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestCatalogDegraded_AFreshCatalogIsNotRefreshedAtEitherStage|TestCatalogDegraded_ZeroLiveKeepsFetchedAtSoTheNextCycleStartRetries|TestWireOrchestratorDeps_TheCatalogRefresherIsWiredAndTheTTLStopsItOnAFreshCatalog' ./cmd/evolve"
---

# Eval: Model currency C3, adopt only live picks

> Pins the two C3 parts that were open after the first C3 landing (plan C3 landing notes, "Open after this step"): the `catalog_refresh_degraded` Signal Center event and the TTL-gated refresh at loop boot. The zero-live guard, `KeepLivePicks` and the shared `LiveCount` predicate stay pinned by their existing tests. Source: inbox item model-currency-c3-adopt-only-live-picks, cycle 2176.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| degraded-event | prior live pick kept and one event per degraded family | 6/10 | `go test -run TestCatalogDegraded_EnforceEmitsOneEventPerFamilyWhoseListFailed ./cmd/evolve` |
| event-shape | registered config.warning, delivered unchanged | 6/10 | `go test -run TestRefreshDegradedEvent_ ./internal/modelcatalog` |
| boot-refresh | once, before the preflight, error is a WARN | 7/10 | `go test -run TestRefreshCatalogAtLoopBoot_ ./cmd/evolve` |
| ttl-and-zero-live | fresh catalog not refreshed, FetchedAt kept | 7/10 | `go test -run TestCatalogDegraded_ZeroLive ./cmd/evolve` |
