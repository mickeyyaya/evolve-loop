# Build Explanation — Cycle 1824

## Build Binding
- Cycle: 1824
- Base SHA: cac6585f1a4c77c7c8f7b4a3e657809b32b77474

## Summary
`interaction.WriteRollup` now writes `interaction-summary.json` through `atomicwrite.Bytes` instead of a fixed `path + ".tmp"` temp file followed by `os.Rename`.

## Rationale
The fixed temp path is the ADR-0049 N14 collision class: two writers of one workspace share `interaction-summary.json.tmp`, so the second rename fails with ENOENT, and a crash or a squatter at that name blocks the write or leaks a stray temp file. `atomicwrite` is the repository's single crash-safe write path; its per-call unique temp file removes the shared name, and it removes its temp file on every failure. The package already depends on `atomicwrite` for rule writes, so no new dependency enters.

## Changed Areas
- `go/internal/interaction/rollup.go` — `WriteRollup` delegates the write to `atomicwrite.Bytes`, keeping the same bytes (indented JSON plus a trailing newline), the same 0644 mode and the no-file-when-empty behavior.
- `docs/architecture/packages/internal-interaction.md` — records that rollup writes also go through `atomicwrite` and names the concurrency predicate that pins it.

## Design Decisions
`atomicwrite.Bytes` is used rather than `atomicwrite.JSON` because `JSON` writes no trailing newline, and the summary's on-disk format keeps its newline. The `Recorder.Record` append path is unchanged: triage dropped it from scope, and the cycle's `TestC1824_004_ConcurrentRecordCallsWriteWholeLedgerLines` predicate shows concurrent records already land as whole lines.

## Verification
The cycle-1824 ACS predicates cover 16 concurrent writers over 100 iterations, a foreign in-flight temp file left untouched, a directory squatting the old temp name, a failed rename that returns its error and leaves no temp file, and an empty workspace that gets no file. `go test -race` passes for `internal/interaction` and `internal/atomicwrite`.

## Compatibility
The output path, content, permissions and the production caller (`go/internal/core/orchestrator.go`) are unchanged. The only visible difference is the temp file's name while a write is in flight.

## Limitations
The inbox item's other parts are not in this build: the absent-only rule test, the stale pointer in `core/cyclerun_correction.go` and the `prompt_cleared` test-inventory row. Triage left them out of this cycle's scope.
