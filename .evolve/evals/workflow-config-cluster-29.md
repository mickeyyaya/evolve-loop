---
score_cap:
  - criterion: "5 workflow-defaults flags absent from flagregistry.Lookup"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -run TestC29_001_WorkflowFlagsAbsentFromRegistry ./acs/cycle29/"
  - criterion: "Registry row count equals 115 after 5-row removal"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -run TestC29_002_RegistryRowCountIs115 ./acs/cycle29/"
  - criterion: "FlagCeiling constant lowered to 115 in registry_ceiling_test.go"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -run TestC29_003_FlagCeilingConstIs115 ./acs/cycle29/"
  - criterion: "No os.Getenv/envchain reads for 5 removed flags in production Go"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -run TestC29_004_NoEnvReadsForRemovedFlags ./acs/cycle29/"
  - criterion: "policy.WorkflowConfig struct + resolver + correct defaults (MaxConsecutiveFails=1, MaxCyclesCap=25, AutoPrune=true)"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -run TestC29_005_WorkflowConfigStructExistsInPolicy ./acs/cycle29/"
  - criterion: "EVOLVE_WORKTREE_PATH not accidentally removed (forbidden-repeat guard)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -run TestC29_007_WorktreePathStillRegistered ./acs/cycle29/"
  - criterion: "Empty WorkflowPolicy{} struct returns all correct defaults from WorkflowConfig() resolver"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -run TestC29_E01_WorkflowConfigEmptyPolicyDefaults ./acs/cycle29/"
---

# Eval: WorkflowConfig Cluster — Cycle 29

> Pins the behavioral contracts for the `workflow-config-cluster-29` task:
> migrating 5 Workflow Defaults env-flag reads (EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS,
> EVOLVE_MAX_CYCLES_CAP, EVOLVE_AUTO_PRUNE, EVOLVE_DIFF_COMPLEXITY_DISABLE,
> EVOLVE_AUDITOR_TIER_OVERRIDE) into `policy.WorkflowConfig` via the Configuration
> Object pattern (bucket 1), identical to DispatchConfig (cycle-28) and
> QuotaResetConfig (cycle-26). Lowering FlagCeiling 120→115 is bound in the same
> diff. Source incident: cycle-29 TDD phase (this cycle).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| registry-absence | 5 flags absent from Lookup | 9/10 | `go test -tags acs -run TestC29_001` |
| row-count | len(All) == 115 | 8/10 | `go test -tags acs -run TestC29_002` |
| ceiling-const | FlagCeiling == 115 | 7/10 | `go test -tags acs -run TestC29_003` |
| no-env-reads | no os.Getenv/envchain literals | 8/10 | `go test -tags acs -run TestC29_004` |
| config-struct | WorkflowConfig struct + resolver + defaults | 9/10 | `go test -tags acs -run TestC29_005` |
| forbidden-repeat | WORKTREE_PATH still registered | 7/10 | `go test -tags acs -run TestC29_007` |
| edge-defaults | empty struct → correct defaults | 7/10 | `go test -tags acs -run TestC29_E01` |
