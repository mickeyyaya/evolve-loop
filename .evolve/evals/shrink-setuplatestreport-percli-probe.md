---
score_cap:
  - criterion: "cmd/evolve.setupLatestReport in go/cmd/evolve/cmd_setup_latest.go is at or under the 50-line size-ratchet ceiling (was 52) and keeps its name"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1754_002_SetupLatestReportWithinSizeRatchetLimit ./acs/cycle1754"
  - criterion: "the module-wide size ratchet passes: probeCLILatest is not a new offender and no listed function is past its offenders.json allowance (the setupLatestReport entry is either removed or left as slack)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1754_003_ModuleSizeRatchetCheckPasses ./acs/cycle1754"
  - criterion: "probeCLILatest(ctx, c, catTiers, lister, fresh) returns the per-CLI row: stale tier with its pair and MapStale, lister error sets Error and skips tier computation, empty listing gives Candidates 0 and no stale tier, catalog tier overrides manifest, fresh[c.CLI] applied; setupLatestReport calls it and no longer calls lister.List inline"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1754_005_ProbeCLILatestContractAndCallerProof ./acs/cycle1754"
  - criterion: "setupLatestReport fan-out is behavior-preserved: one row per ready CLI in order, parallel probes, per-row failure isolation, the probe timeout bound, and every existing TestSetupLatestReport_* case passes"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1754_006_SetupLatestReportFanOutBehaviorPreserved ./acs/cycle1754"
  - criterion: "the touched cmd/evolve files are gofmt clean and go vet ./cmd/evolve/ exits 0"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1754_007_TouchedFilesGofmtAndVetClean ./acs/cycle1754"
---

# Eval: Shrink cmd/evolve.setupLatestReport by extracting probeCLILatest

> Pins the cycle-1754 size-ratchet shrink of `setupLatestReport` (go/cmd/evolve/cmd_setup_latest.go,
> 52 lines, listed at 52 in go/internal/sizeratchet/offenders.json). The per-CLI goroutine body
> becomes `probeCLILatest`, called from the unchanged WaitGroup fan-out. The contract tests
> call the probe directly (happy, error and empty-listing branches), the seven existing
> `TestSetupLatestReport_*` tests pin the fan-out, and the caller check keeps the probe
> reachable from its production caller. An empty listing still names a real current model
> unverified, as it did before the extraction. Source: cycle 1754 scout/triage top_n.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| size-ceiling | setupLatestReport <= 50 lines, name kept | 8/10 | `TestC1754_002_SetupLatestReportWithinSizeRatchetLimit` |
| ratchet-clean | module-wide `sizeratchet.Check` passes | 7/10 | `TestC1754_003_ModuleSizeRatchetCheckPasses` |
| contract+negative+caller | probeCLILatest branches + caller proof | 9/10 | `TestC1754_005_ProbeCLILatestContractAndCallerProof` |
| fan-out regression | all 7 `TestSetupLatestReport_*` pass | 8/10 | `TestC1754_006_SetupLatestReportFanOutBehaviorPreserved` |
| build-clean | gofmt + go vet | 6/10 | `TestC1754_007_TouchedFilesGofmtAndVetClean` |
