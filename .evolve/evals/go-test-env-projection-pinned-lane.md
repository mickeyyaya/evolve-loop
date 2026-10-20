---
score_cap:
  - criterion: "every production go test spawner builds its env from ipcenv.GoTestEnv or ciparity.CIEnv, pkg/acsassert included"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestGoTestSpawners_EveryProductionSpawnerUsesTheNamedProjection$' ./internal/ipcenv"
  - criterion: "the pin flags a planted spawner and accepts the named projections"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestGoTestSpawners_APlantedSpawnerThatBuildsItsEnvAnyOtherWayIsFlagged$|^TestGoTestSpawners_ASpawnerThatUsesANamedProjectionIsAccepted$' ./internal/ipcenv"
  - criterion: "the pin flags a spawner whose projection call is a decoy or whose env re-adds the ambient os.Environ()"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestGoTestSpawners_' ./internal/ipcenv"
  - criterion: "the pin's package is in repocontract.Packages()"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestPackages_HoldEveryTestThatReadsTheWholeTree$' ./internal/repocontract"
  - criterion: "pkg/acsassert GoTests runs its child go test without the caller's EVOLVE_ keys"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags integration -run '^TestGoTests_TheChildGoTestSeesNoEvolveKeyOfItsCaller$' ./pkg/acsassert"
---

# Eval: pin one go-test environment projection across every production go test spawner

> Pins inbox item go-test-env-projection-pinned-lane (cycle 1978). A new
> exec.Command("go", "test", ...) or sysexec go test spawner that inherits the
> lane's EVOLVE_FLEET and cycle-state keys false-REDs env-sensitive packages
> (the 2026-09-14 and cycle-1700 class). The whole-tree AST test in
> internal/ipcenv makes the rule a gate.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| whole-tree pin | no production go test spawner outside the projection | 4/10 | `go test -run TestGoTestSpawners_EveryProductionSpawnerUsesTheNamedProjection ./internal/ipcenv` |
| planted spawner | the pin goes red on a planted spawner | 6/10 | `go test -run TestGoTestSpawners_APlantedSpawner... ./internal/ipcenv` |
| decoy projection | a projection call that never reaches the env, or an env that appends os.Environ() back, still fails the pin | 6/10 | `go test -run TestGoTestSpawners_ ./internal/ipcenv` |
| pack membership | the tree-reading pin joins repocontract.Packages() | 6/10 | `go test -run TestPackages_HoldEveryTestThatReadsTheWholeTree ./internal/repocontract` |
| acsassert | the child go test sees no EVOLVE_ key | 6/10 | `go test -tags integration -run TestGoTests_TheChildGoTestSeesNoEvolveKeyOfItsCaller ./pkg/acsassert` |
