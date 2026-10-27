# Build Explanation — Cycle 2056

## Build Binding
- Cycle: 2056
- Base SHA: 1d361aef266b33532296574243e4b77a0e7c44aa

## Summary
Each llm-calls row now records the cycle, run id, profile and a derived purpose of the request that produced it. At the enforce stage, a row from a cycle workspace with no cycle number fires TOKENS_ROW_UNATTRIBUTED.

## Rationale
The request already carries the attribution. The bridge copies it into the row, so no new data source exists. Old rows lack the new keys, and the keys use omitempty, so old rows decode unchanged.

## Changed Areas
- `go/internal/llmcalls/record.go` — adds the Cycle, RunID, Profile and Purpose fields and the three purpose names.
- `go/internal/bridge/attempt_telemetry.go` — copies the request attribution into the row, derives the purpose, and signals an unattributed cycle row at enforce.
- `go/internal/bridge/engine.go` — adds the TokenAttributionStage setting to Deps.
- `go/internal/bridge/signal.go` — declares and registers the TOKENS_ROW_UNATTRIBUTED code.
- `go/internal/signalcenter/event.go` — adds the tokens module to the closed module set.

## Design Decisions
A correction directive has priority over a retry number. The correction is the more specific cause of the call. An empty stage means shadow. In shadow, no signal fires until the stage is set to enforce.

## Verification
The bridge, llmcalls and signalcenter tests and the five cycle 2056 predicates cover the row fields, the purpose derivation, old-row decoding and the stage gate.

## Compatibility
Old rows decode and import byte for byte. The row schema version is unchanged.

## Limitations
No policy setting feeds TokenAttributionStage yet, so production runs stay at shadow until a later item wires it.
