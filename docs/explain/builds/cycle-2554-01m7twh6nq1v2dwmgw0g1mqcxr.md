# Build Explanation — Cycle 2554

## Build Binding
- Cycle: 2554
- Base SHA: f25d49e4ae89c9d1c528d1c4fbc1687743635999

## Summary
This build adds `evolve stage verify` and `evolve stage report`, the `internal/stageevidence` scorer and chain-verification package, and the decision version digest over the decider closure and policy subtree.

## Rationale
Putting chain checks, version hashing, and scoring into `internal/stageevidence` behind a `ChainReader` port keeps the verb independent of `core`. It also lets the verb score fixture chains without waiting for the K2 recorder.

## Changed Areas
- `go/internal/stageevidence/score.go` — implements `Link`, `VerifyChain`, `Score`, `Verify`, `Scorer`, `DefaultScorer`, `ScorerFor`, and the typed chain errors (`ErrBrokenChain`, `ErrEmptyChain`, `BrokenChainError`, `ErrorCode`).
- `go/internal/stageevidence/version.go` — implements `DecisionVersion` to hash `go.mod` directives, the module-local decider closure, and the canonical JSON policy subtree.
- `go/internal/stageevidence/chainfile.go` — implements `ChainReader` and the `FileChain` adapter that validates the whole chain file before it filters records for one stage key.
- `go/internal/regressiontia/regressiontia.go` — exports `ModuleLocalReach` so `stageevidence` reuses the existing module-local BFS closure walk instead of duplicating it.
- `go/cmd/evolve/cmd_stage.go` — implements `runStage`, `runStageVerify`, and `runStageReport` with text and JSON output and typed exit codes.
- `go/cmd/evolve/registry.go` — registers the `stage` subcommand in `commands`.
- `go/.apicover-enforce` — enrolls `./internal/stageevidence` in the API coverage gate.
- `docs/architecture/packages/internal-stageevidence.md` — documents the exports and invariants of `internal/stageevidence`.

## Design Decisions
`DecisionVersion` reuses `regressiontia.ModuleLocalReach` over non-test package imports instead of copying a second BFS loop. `FileChain` sets a preverified flag after checking the full multi-key chain so `Verify` accepts gaps between keys. `runStageReport` records an entry error if a fixture root lacks a closure package, whereas `runStageVerify` fails closed.

## Verification
Unit tests in `go/internal/stageevidence`, `go/internal/regressiontia`, `go/cmd/evolve`, and all nine `go/acs/cycle2554` predicates run without skips, and `apicover -enforce` passes on both `internal/stageevidence` and `internal/regressiontia`.

## Compatibility
No existing CLI commands, `gatecontract` types, or `gateregistry` entries changed.

## Limitations
With no `--chain` flag, `evolve stage verify` and `evolve stage report` read an empty chain (`empty-chain`, `not ready`) until the K2 recorder and ledger adapter land.
