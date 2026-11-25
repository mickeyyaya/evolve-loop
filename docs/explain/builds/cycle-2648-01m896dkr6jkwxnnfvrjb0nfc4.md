# Build Explanation — Cycle 2648

## Build Binding
- Cycle: 2648
- Base SHA: 81ffbbe51c994ef45901e624edc7c57647a6bab8

## Summary
`stageevidence` now records a typed `NotReadyReason` on `Result` when a conflict-dependent stage key lacks measured routes. Both `evolve stage verify` and `evolve stage report` print this reason in text and JSON.

## Rationale
Without `NotReady`, `Result` only exposed `Ready` as a boolean and left operators blind to unmeasured conflict classes. One shared predicate, `DependsOnConflict`, defines which stage keys require `Outcome.RoutesMeasured` and avoids inferring dependencies from per-record fields.

## Changed Areas
- `go/internal/stageevidence/score.go` — adds `NotReadyReason`, `ReasonConflictsUnmeasured`, `Outcome.RoutesMeasured`, `Result.NotReady`, and `DependsOnConflict`. `Score` sets `NotReady` and clears `Ready` when a conflict-dependent key lacks `RoutesMeasured`.
- `go/cmd/evolve/cmd_stage.go` — prints `not ready: <reason>` in `writeStageVerifyText` when `NotReady` is non-empty and adds `not_ready=<reason>` to each row in `runStageReport`.

## Design Decisions
`DependsOnConflict` states in one place that only `held_proposals.stage` depends on the conflict class. `Score` calls `DependsOnConflict(r.Key)` per counted decision rather than inspecting `would_hold` fields, so keys such as `gc.sentinel.hold.stage` stay unaffected.

## Verification
`gofmt -l .`, `go vet ./...`, `go test -tags acs -count=1 ./acs/cycle2648`, and `go test -count=1 ./internal/stageevidence/... ./cmd/evolve` verify both the typed reason logic and CLI output.

## Compatibility
Existing JSON field names on `Result` and `Outcome` stay unchanged, but `Result.Ready` for `held_proposals.stage` now also requires `Outcome.RoutesMeasured` on every counted decision. `NotReady` and `RoutesMeasured` use `omitempty`, so JSON output omits false or empty values.

## Limitations
Only `held_proposals.stage` returns true from `DependsOnConflict`, and future conflict-dependent keys must register there. Because no production writer sets `RoutesMeasured` yet (`Seal.Outcome` leaves it false), `held_proposals.stage` cannot become ready in production.
