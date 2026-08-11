# Eval: clihealth-bench-entry-coverage

## Task

Add unit tests for the uncovered functions in `go/internal/clihealth/clihealth.go`:
- `Benchable` (0% → 100%)
- `NewBenchEntry` (0% → 100%)
- `firstLine` (0% → 100%)
- `truncateRunes` (0% → 100%)

## Acceptance Criteria

### AC1: Benchable tested with positive and negative cases [code]

```bash
cd go && go test ./internal/clihealth/... -run TestBenchable -v -count=1
```

Expected: `PASS` with at least one test verifying `Benchable("rate_limit") == true` and `Benchable("auth_recheck") == false`.

### AC2: NewBenchEntry tested — strike accumulation and truncated evidence [code]

```bash
cd go && go test ./internal/clihealth/... -run TestNewBenchEntry -v -count=1
```

Expected: `PASS`; test verifies `Entry.Strikes == prev.Strikes + 1`, `Evidence` is non-empty and ≤ 160 runes, and `BenchedUntil` is in the future.

### AC3: firstLine and truncateRunes covered [code]

```bash
cd go && go test ./internal/clihealth/... -run "TestFirstLine|TestTruncateRunes" -v -count=1
```

Expected: `PASS`; tests cover single-line (no newline), multi-line (stops at first `\n`), and multi-byte rune truncation edge case.

### AC4: Coverage floor — clihealth ≥ 90% [code]

```bash
cd go && go test ./internal/clihealth/... -cover -count=1 2>&1 | grep -E "coverage: [0-9]"
```

Expected: output contains `coverage: 9[0-9]\.[0-9]%` or `coverage: 100.0%` (≥ 90%).

### AC5: No regressions [code]

```bash
cd go && go test ./internal/clihealth/... -count=1
```

Expected: `ok` with zero failures.
