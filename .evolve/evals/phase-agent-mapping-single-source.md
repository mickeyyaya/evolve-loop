---
score_cap:
  - criterion: "Every llm catalog phase resolves through PhaseSpec.AgentName() to a tracked profile and persona; build, audit and tdd resolve to builder, auditor and tdd-engineer"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRealRegistry_CatalogPhaseResolvesToTrackedProfile$' ./internal/phasespec"
  - criterion: "cliroute resolves a phase's agent from the catalog alone; the built-in contract table no longer answers"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1866_002' ./acs/cycle1866"
  - criterion: "cliroute still maps the real spine phases to their tracked profiles and refuses an unknown agents key"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1866_003' ./acs/cycle1866"
  - criterion: "phaseinventory projects the catalog AgentName, which resolves to a tracked profile for the spine"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1866_004' ./acs/cycle1866"
  - criterion: "existing cliroute, phaseinventory, phasespec and phasecontract tests stay green"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/cliroute/... ./internal/phaseinventory/... ./internal/phasespec/... ./internal/phasecontract/..."
---

# Eval: one source for a phase's agent

> Pins the single source of the phase-to-agent mapping. Before cycle 1866 the registry left `agent` empty for build, audit and tdd, so `PhaseSpec.AgentName()` returned `evolve-build`, `evolve-audit` and `evolve-tdd`. These names have no profile and no persona. `internal/cliroute` read the built-in contracts in `internal/phasecontract` as a second table. Source: inbox item `phase-agent-mapping-single-source` (console, CLI routing table L1a, 2026-10-05), cycle 1866.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| registry-resolves | every llm phase has a tracked profile and persona | 8/10 | `TestRealRegistry_CatalogPhaseResolvesToTrackedProfile` |
| catalog-alone | the catalog agent wins over the contract table | 8/10 | `TestC1866_002` |
| preservation | real spine still routes; unknown key refused | 7/10 | `TestC1866_003` |
| inventory | inventory shows the catalog agent | 6/10 | `TestC1866_004` |
| no-regression | package suites green | 7/10 | package `go test` |
