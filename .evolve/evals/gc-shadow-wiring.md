# Eval: gc-shadow-wiring

## Task
Wire the `EVOLVE_GC=off|shadow|enforce` flag into the evolve loop startup.

## Acceptance Criteria

### AC1 — Flag registered in flagregistry [code]
```bash
grep -c '"EVOLVE_GC"' go/internal/flagregistry/registry_table.go
```
Expected: `1`
Grader: `[code]` — exact match

### AC2 — control-flags.md contains EVOLVE_GC entry [code]
```bash
grep -c 'EVOLVE_GC' docs/architecture/control-flags.md
```
Expected: output ≥ 1 (non-zero)
Grader: `[code]` — numeric threshold

### AC3 — flags check passes (no drift) [code]
```bash
cd go && go run ./cmd/evolve flags check
```
Expected: exit code 0
Grader: `[code]` — exit-code check

### AC4 — Shadow mode writes manifest without mutations [code]
```bash
# Build the binary and run a unit test that exercises the shadow path:
cd go && go test -run TestGCShadow ./cmd/evolve/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]` — test pass check

### AC5 — Off mode (default) writes no manifest [code]
```bash
cd go && go test -run TestGCOff ./cmd/evolve/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]` — test pass check

### AC6 — Invalid EVOLVE_GC value does not panic [code]
```bash
cd go && go test -run TestGCInvalidMode ./cmd/evolve/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]` — test pass check

### AC7 — Enforce mode calls Apply, shadow does not [code]
```bash
cd go && go test -run TestGCEnforce ./cmd/evolve/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]` — test pass check

### AC8 — All tests pass (no regression) [code]
```bash
cd go && go test ./... 2>&1 | grep -c "^ok"
```
Expected: ≥ 129
Grader: `[code]` — numeric threshold

## Negative Cases

### N1 — Manifest absent when EVOLVE_GC=off [code]
The test `TestGCOff` must assert that no `gc-shadow-manifest.json` file is written.
Fake that passes: an impl that always writes the file would fail this.

### N2 — Protected path (quarantine/) never appears in manifest [code]
```bash
cd go && go test -run TestGCProtectedPath ./internal/gc/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]`

## Edge Cases

### E1 — Missing .evolve/runs dir: shadow completes with empty manifest [code]
```bash
cd go && go test -run TestGCShadowMissingRunsDir ./cmd/evolve/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]`

### E2 — Current live run never appears in manifest [code]
```bash
cd go && go test -run TestGCShadowLiveRunExcluded ./cmd/evolve/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]`
