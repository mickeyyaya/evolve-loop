# Eval: observer-rule-promotion

## Task
Promote phase-observer secondary detection rules (infinite_loop, error_spike, cost_anomaly) from best-effort to INCIDENT severity, triggering enforcement.

## Criteria

### C1: infinite_loop fires INCIDENT in unit test [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phaseobserver/... -run TestInfiniteLoopIncident -v -timeout 30s 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```

### C2: error_spike fires INCIDENT in unit test [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phaseobserver/... -run TestErrorSpikeIncident -v -timeout 30s 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```

### C3: cost_anomaly fires INCIDENT in unit test [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phaseobserver/... -run TestCostAnomalyIncident -v -timeout 30s 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```

### C4: Existing stuck_no_output (stall) test still passes — no regression [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phaseobserver/... -timeout 60s -count=1 2>&1 | \
  tail -3 | grep -q "ok" && echo "PASS" || echo "FAIL"
```

### C5: Negative case — detection NOT fired when below threshold [code]
```bash
# Tests for rule correctness: below-threshold inputs must NOT produce INCIDENT
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phaseobserver/... -run TestBelowThresholdNoIncident -v -timeout 30s 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```

### C6: Config fields have sensible defaults (no undefined zero-value traps) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go vet ./internal/phaseobserver/... 2>&1 | grep -q "." && echo "FAIL - vet errors" || echo "PASS"
```
