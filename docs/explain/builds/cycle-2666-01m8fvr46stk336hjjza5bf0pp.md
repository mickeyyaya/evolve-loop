# Build Explanation — Cycle 2666

## Build Binding
- Cycle: 2666
- Base SHA: 59e7b0750b9210903d273be167b7434d045f8691

## Summary
The debugger decision parser and persona output contract now use `rationale` instead of `reasoning`. The parser keeps one release of fallback read support for legacy `reasoning`. The build phase failed-approach JSON template now uses `root_cause` instead of `reasoning`.

## Rationale
The debugger decision struct already held `root_cause` beside `reasoning`. That naming blurred why a ship failed and why a recovery action was chosen. Renaming the field to `rationale` with `reasoning` fallback avoids breaking legacy `debug-decision.json` files. It also aligns the parser, persona, and build documentation.

## Changed Areas
- `go/internal/phases/debugger/debugger.go` — adds `signalRationale` (`"debugger.rationale"`), `decision.Rationale` (`json:"rationale"`), `recordOptionalSignals`, and `resolveRationale` so `Classify` prefers trimmed `Rationale` and falls back to trimmed `Reasoning` without exceeding its function-size ceiling.
- `agents/evolve-debugger.md` — renames the `"reasoning"` field to `"rationale"` in the `## Output contract` JSON example so the persona matches the parser contract.
- `skills/loop/phase3-build.md` — renames `"reasoning"` to `"root_cause"` in the `failedApproaches` JSON template under `## Post-Builder Handling`.

## Design Decisions
Keeping `decision.Reasoning` alongside `decision.Rationale` inside a single `resolveRationale` helper preserves legacy read support in one place and lets a later release drop the field cleanly. Publishing the resolved text on `debugger.rationale` only when non-empty avoids emitting blank signals on decisions that omit both fields.

## Verification
`gofmt -l`, `go vet`, `go test -count=1 ./internal/phases/debugger/...`, and `go test -tags acs -count=1 ./acs/cycle2666` pass, including `TestDecisionRationale` and `TestC2666_001` through `TestC2666_003`.

## Compatibility
Decisions that contain only the legacy `"reasoning"` key still parse into `debugger.rationale` when `rationale` is absent or whitespace. Existing `debugger.action`, `debugger.rerun_phase`, and `debugger.root_cause` signals remain unchanged.

## Limitations
Legacy `"reasoning"` read support remains active until a follow-up cleanup removes the fallback field after one release.
