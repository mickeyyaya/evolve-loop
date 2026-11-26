# Build Explanation — Cycle 2652

## Build Binding
- Cycle: 2652
- Base SHA: b2feb988ef659e9d8c0cccc5986d506be79cc37c

## Summary
The production lane batcher and L2 batch test now run a tier-aware gate (`l2Gate`) instead of a nil or no-op gate.

## Rationale
Reusing `l2Checkout` and `integration.Command` executes `go build` and `go vet` inside an isolated tip checkout and returns `*integration.GateRed` on red steps without modifying `integration.Composer`.

## Changed Areas
- `go/cmd/evolve/cmd_integration_l2.go` — added `l2Gate` and `gateStep`, wired `l2Gate` into `batchTest`, and removed `l2NoGate`.
- `go/cmd/evolve/cmd_loop_driver.go` — wired `l2Gate(l2Checkout(ports.Repo, b.cfg.EvolveDir), env.Run)` into `Composer.Gate` inside `productionEnforceActor`.
- `go/cmd/evolve/cmd_stage_chain.go` — resolved a symlinked `ledger.jsonl` in `stageLedgerReader` so worktrees read the real ledger store and avoid `ledger-broken`.
- `docs/architecture/packages/internal-integration.md` — documented `l2Gate`, removed the stale `l2NoGate` and missing-gate statements, and recorded the measured soak `BatchTest cost per batch: 944 ms` for 4 members.

## Design Decisions
`l2Gate` runs `go build ./...` for tiers `T1` and `T2`, and runs `go build ./...` and `go vet ./...` for `T3` and `T4`. It wraps command runner errors with `acsrunner.HostFailure` and returns checkout errors directly. One helper in `cmd_integration_l2.go` serves both `batchTest` and `productionEnforceActor`, instead of two separate gate implementations or a package flag.

## Verification
All six `cycle2652` ACS predicates pass. The `TestL2Gate_*`, `TestGateJourney_*`, `TestGateSoak_*`, `TestL2Adapters_*`, `TestProductionDriver_*`, `TestIntegrationTrain*`, `TestEveryStageHasScorerAndVerdict`, and `TestStageLedgerReaderFollowsSymlinkedLedgerToItsStore` tests, `go vet`, and `gofmt -l .` pass. `l2Gate`, `batchTest`, `productionEnforceActor`, and `stageLedgerReader` reach 100% statement coverage.

## Compatibility
`integration.Composer` and `integration.LaneBatcher` keep their existing interfaces and error semantics (`*integration.GateRed` for red steps and `acsrunner.HostFailure` for runner errors).

## Limitations
The gate verifies `go build ./...` and `go vet ./...` only and leaves full test and repository contract checks to `BatchTest` and CI.
