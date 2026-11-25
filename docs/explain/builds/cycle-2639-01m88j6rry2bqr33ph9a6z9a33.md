# Build Explanation — Cycle 2639

## Build Binding
- Cycle: 2639
- Base SHA: c28bcf9db03559915b6962540ec821c197628420

## Summary
The routed-source guard in `stageevidence` now validates each emit site against a registry of scorer tests. `Seal` carries `Routed` into `Outcome` so `Score` can reach `ClassConflict`.

## Rationale
The earlier guard failed whenever any source file filled `Outcome.Routed`, which blocked changes that wire routed items into `Seal.Outcome`. The new registry maps `(File, Kind)` pairs to named `Test*` functions. An unregistered emit site or missing scorer test fails the guard, while registered sites with live tests pass. Adding `Routed []string` to `Seal` and `SealInput` lets `Seal.Outcome` emit routed items into the scorer while keeping `VerifyStored` round-trip consistent.

## Changed Areas
- `go/internal/stageevidence/seal.go` — adds `Routed []string` (`json:"routed,omitempty"`) to `Seal` and `SealInput`, copies `in.Routed` in `BuildSeal`, forwards `seal.Routed` in `VerifyStored`, and emits `Routed: s.Routed` from `Seal.Outcome`.

## Design Decisions
`routedEmitRegistry` keys on `(File, Kind)` rather than `File` alone. A second emit pattern in an already-registered file must also register its own scorer test. `routedRegistryViolations` checks every registry entry against `scorerTestNames(".")` and every scan finding against the registry. A registered site absent from the tree still passes once its test exists, and any missing test fails. The guard keeps the `"F4"` label in both pass and fail logs to preserve compatibility with cycle 2620 predicates.

## Verification
`gofmt -l ./internal/stageevidence/`, `go vet ./internal/stageevidence/... ./internal/adapters/ledger/...`, `go test -count=1 ./internal/stageevidence/... ./internal/adapters/ledger/...`, `go test -tags acs -count=1 -v ./acs/cycle2639 ./acs/cycle2620`, and `evolve acs suite --cycle 2639` verify the implementation.

## Compatibility
`Seal.Routed` uses `json:"routed,omitempty"`, so existing seals without routed items encode identically and pass `VerifyStored`. The `Seal.Outcome` signature and `StageChainReader.addScope` caller in `go/internal/adapters/ledger/stage_evidence.go` stay unchanged.

## Limitations
Only `internal/stageevidence/seal.go` exists as a routed emit site on this tree. Emit sites from PR 1735 must register in `routedEmitRegistry` with scorer tests in `stageevidence_test`.
