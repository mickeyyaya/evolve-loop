# Eval: interaction-error-branch-coverage

## Task
Add unit tests for three uncovered error/edge branches in `go/internal/interaction/interaction.go`:
1. `ledgerPath` empty-phase fallback → "unknown" suffix
2. `neutralize` second rune-cap cut (Digest multi-line result > 200 runes)
3. `Record` with an unwritable workspace path (OpenFile error swallowed, in-memory record preserved)

## Acceptance Criteria

### AC1: `ledgerPath` empty-phase produces "unknown-interactions.ndjson"
```bash
cd go && go test -v -run TestRecord_EmptyPhaseFallsBackToUnknownLedger ./internal/interaction/...
```
[code] must exit 0 and print `PASS`

### AC2: `neutralize` multi-line large payload capped at ≤200 runes
```bash
cd go && go test -v -run TestNeutralize_MultiLineLargePayload ./internal/interaction/...
```
[code] must exit 0 and print `PASS`

### AC3: `Record` with bad workspace swallows error and preserves in-memory record
```bash
cd go && go test -v -run TestRecord_InvalidWorkspaceSwallowsFileError ./internal/interaction/...
```
[code] must exit 0 and print `PASS`

### AC4: Coverage improvement on `interaction.go`
```bash
cd go && go test -coverprofile=/tmp/cover_int.out ./internal/interaction/... && go tool cover -func=/tmp/cover_int.out | grep "interaction.go" | awk '{sum+=$3; n++} END {print (n>0 ? sum/n : 0)}'
```
[code] must output a value ≥ 85 (ledgerPath ≥ 90%, neutralize ≥ 90%)

### AC5 (negative): `ledgerPath` with non-empty phase does NOT produce "unknown" suffix
```bash
cd go && go test -v -run TestRecord_EveryInjectionKindProducesOutcome ./internal/interaction/...
```
[code] must exit 0; the existing test still passes — no regression in the normal path

### AC6 (edge): full test suite still green after adding tests
```bash
cd go && go test ./internal/interaction/...
```
[code] must exit 0
