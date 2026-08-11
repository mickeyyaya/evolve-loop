# Eval: add-phase-latency-health-signal

## Metadata
- slug: add-phase-latency-health-signal
- cycle: 180
- task: Add signal 12 `phase_latency` to cycle-health that reads phase-timing.json and flags phases exceeding a configurable ceiling

## Context

`go/internal/cyclehealth/cyclehealth.go` implements 11 integrity signals. Signal 6
(`velocity`) uses coarse ledger timestamps to detect slow phases but has no access to
the fine-grained `duration_ms` values in `phase-timing.json` (written by the orchestrator
post-cycle). Adding signal 12 `phase_latency` gives operators an actionable per-phase
latency alert: any phase whose `duration_ms` exceeds `EVOLVE_PHASE_LATENCY_CEILING_S * 1000`
(default 900 s = 15 min) emits a WARN anomaly.

The signal is gracefully skipped when `phase-timing.json` is absent (cycle may not have
written it yet, or pre-v12 workspace).

## Acceptance Criteria

### AC-1: `phase_latency` signal name present in cyclehealth.go [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -q "phase_latency" "$REPO_ROOT/go/internal/cyclehealth/cyclehealth.go" \
  || { echo "RED: phase_latency signal not found in cyclehealth.go"; exit 1; }
echo "GREEN: phase_latency signal present"
```

### AC-2: signalNames returns 12 entries (test updated) [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go" && go test ./internal/cyclehealth/... -run TestSignalNames -v -count=1 -timeout 30s 2>&1 \
  | grep -E "--- PASS.*SignalNames|--- FAIL.*SignalNames" \
  | grep -q PASS \
  || { echo "RED: TestSignalNames failed (likely still expects 11)"; exit 1; }
echo "GREEN: TestSignalNames passes (expects 12)"
```

### AC-3: phase_latency signal reads phase-timing.json [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -q "phase-timing.json\|phase_timing\|phaseTimingPath" "$REPO_ROOT/go/internal/cyclehealth/cyclehealth.go" \
  || { echo "RED: cyclehealth.go does not reference phase-timing.json"; exit 1; }
echo "GREEN: cyclehealth.go references phase-timing.json"
```

### AC-4: Slow phase triggers WARN anomaly (test) [code]
A test must write a phase-timing.json with duration_ms exceeding the ceiling and verify
a `phase_latency` WARN anomaly is emitted.
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go" && go test ./internal/cyclehealth/... -run "PhaseLatency\|SlowPhase\|Latency" \
  -v -count=1 -timeout 30s 2>&1 \
  | grep -E "--- PASS|--- FAIL" \
  | grep -v FAIL \
  | grep -q PASS \
  || { echo "RED: no passing PhaseLatency/SlowPhase test found"; exit 1; }
echo "GREEN: phase_latency WARN test passes"
```

### AC-5: Missing phase-timing.json does not produce anomaly (graceful skip) [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go" && go test ./internal/cyclehealth/... -run "PhaseLatency.*Miss\|MissingTiming\|NoTimingFile" \
  -v -count=1 -timeout 30s 2>&1 \
  | grep -E "--- PASS|--- FAIL" \
  | grep -v FAIL \
  | grep -q PASS \
  || { echo "RED: no passing MissingTiming/NoTimingFile test found"; exit 1; }
echo "GREEN: missing phase-timing.json gracefully skipped"
```

### AC-6: All cyclehealth tests still pass (no regression) [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go" && go test ./internal/cyclehealth/... -count=1 -timeout 60s 2>&1 \
  | tail -3 \
  | grep -q "^ok" \
  || { echo "RED: ./internal/cyclehealth/... tests failed"; exit 1; }
echo "GREEN: ./internal/cyclehealth/... all pass"
```

### AC-7: EVOLVE_PHASE_LATENCY_CEILING_S documented in CLAUDE.md [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -q "EVOLVE_PHASE_LATENCY_CEILING_S" "$REPO_ROOT/CLAUDE.md" \
  || { echo "RED: EVOLVE_PHASE_LATENCY_CEILING_S not in CLAUDE.md"; exit 1; }
echo "GREEN: EVOLVE_PHASE_LATENCY_CEILING_S documented"
```
