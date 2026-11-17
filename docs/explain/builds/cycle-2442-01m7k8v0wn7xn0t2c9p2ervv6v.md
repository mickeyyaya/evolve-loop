# Build Explanation — Cycle 2442

## Build Binding
- Cycle: 2442
- Base SHA: d5f670867399c94f855855482b1094e18be8744a

## Summary
The console train now runs the batch test (L2) before it opens the batch PR. An L2 red ejects the later member with the cause in its brief. A host kill ejects nobody.

## Rationale
`BatchTest.Check` had no production caller, so a batch landed on CI alone. The train calls `Check` between the compose and `CI.Open` when the stage of the policy is enforce. A shadow stage runs no seam, so the default behavior does not change.

## Changed Areas
- `go/internal/acsrunner/runner.go` — adds `HostFailure`, `IsHostFailure` and `RunIn`. A signal kill, a cancelled context and a start error are host failures.
- `go/internal/integration/batchtest.go` — adds `BatchTest.Pack`. A host failure of a predicate run ejects nobody.
- `go/internal/integration/l2pack.go` — adds `PackStep`, `Checkout`, `Command` and `L2Pack`, the six checks of the L2 row.
- `go/internal/integration/train.go` — adds `Train.L2`. `batcher.compose` calls `Check`, keeps the landed members and moves the batch branch to the tested tip.
- `go/cmd/evolve/cmd_integration_l2.go` — builds the production seams: a git checkout, the affected set, the package test and the predicate run. It also adds a tip verdict store and reads the stage of `policy.json`.
- `go/cmd/evolve/cmd_integration.go` — adds the `Run` command port to `integrationEnv`.
- `go/cmd/evolve/cmd_integration_batch.go` — `env.train` sets `Train.L2`.
- `go/internal/acsrunner/runin_test.go` — names `RunIn` in a real assertion.
- `go/internal/integration/l2pack_test.go` — names the pack commands and the checkout release.

## Design Decisions
`Check` composes in shadow, so its composer gate never runs. The production gate is a no-op that satisfies the enforce precondition. A pack step that cannot run is a host failure, never a red.

## Verification
The frozen tests of three packages pass. The four ACS predicates of cycle 2442 pass. The module stays at go 1.23 in `go/go.mod`.

## Compatibility
A shadow stage, which is the default, runs no L2 seam. The `acsrunner.Run` signature is unchanged.

## Limitations
The two cover steps run `make test-integration`, so an enforce batch is slow. The host failure test changes the directory with `os.Chdir` and restores it in `t.Cleanup`, so it needs no go 1.24 feature.
