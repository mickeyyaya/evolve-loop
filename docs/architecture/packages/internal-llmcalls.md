# internal/llmcalls

> The proposed attribution fields of the row (cycle, run id, profile, round purpose) and the turn sidecar: [token-telemetry-2026-10](../../plans/token-telemetry-2026-10.md). The research: [token-telemetry-2026-10](../../research/token-telemetry-2026-10.md).

## Purpose

`internal/llmcalls` owns the durable ledger of model-try records and the performance index derived from it. It holds no provider or orchestration logic. Producers supply finalized dispatch facts, and readers consume the same additive record schema.

## Design

- **Additive schema.** `Record` keeps the historical fields so existing readers decode new lines unchanged. The added fields make the measurement coverage and the real dispatch identity explicit. Outcome fields are pointers, so legacy input keeps "missing" apart from zero.
- **Model identity.** `Model` is the legacy requested-selector field. New consumers use the requested and dispatched pair. `ModelIdentity` returns only a verified dispatched selector. A legacy model value stays as requested context and is never promoted to a concrete model identity (`UnknownModel` is the display value).
- **Dispatch source.** `Dispatch*` values say how the selector reached the provider: process arguments, an interactive command, the positional argument or the CLI default. Other values are a resumed session, not started and unknown. Historical rows without provenance are `DispatchLegacyUnverified`.
- **Timing scope.** `TimingBridgeDispatch` means the duration covers the shared Bridge launch pipeline through driver completion. It excludes token enrichment and the ledger append. First-output timing is optional on purpose, because the bridge has no source that means the same thing for headless and REPL drivers.
- **Usage status.** `UsageStatus` tells a measured zero from missing or partial token evidence. Its string values are part of the durable record contract.
- **Append.** `Append` serializes writers across goroutines and processes and does one append write. If the previous tail is torn, it is separated first, so it cannot consume the new JSON object. The scanner buffer has room for the newline after the largest record that `Append` accepts.
- **Read.** `Read` decodes valid bounded records and skips malformed lines. An oversized line returns the valid prefix and a scanner error with context. `ReadResult` keeps the valid records and makes degraded input visible.
- **Import.** `Import` copies valid source records through the same serialized append path. `call_id` is the primary identity. A legacy record uses its exact trimmed JSON bytes. Repeating a salvage is idempotent and rewrites no history.
- **Valid record.** `validRecord` is the minimum historical contract: every real row names its phase. This rejects JSON null, empty objects and unrelated envelopes, and it adds no requirement for legacy data.
- **Aggregate.** `Aggregate` builds a deterministic performance index and persists no second copy. A `Performance` group is one comparable model and outcome. Token volumes include every valid count. The output rate uses only complete measurements with a positive duration of the try. Legacy labels stay in `RequestedModel`.
- **Try fields (ADR-0142 L0).** `Rung`, `FailureClass`, `Fingerprint`, `Effort`, `ServedModel`, `Context`, `LadderStage` and `LadderWould` are the try record of [retry-ladder §6](../retry-ladder.md#6-the-try-record). Each field is optional, so an old line decodes unchanged. `ServedModel` is the model that the CLI reports, never the tier alias of `Model`.
- **Call id.** `NewCallID` makes an opaque id that is unique in the process, with no third-party UUID dependency. Random bytes prevent collisions across processes. The timestamp, the pid and a monotonic sequence keep it unique if the random source fails.

## Invariants

- A torn tail never swallows the next record. Pinned by `store_test.go`.
- Repeating `Import` adds no record. Pinned by `store_test.go`.

## Findings

- None open.
