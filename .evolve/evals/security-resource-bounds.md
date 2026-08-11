# Eval: security-resource-bounds
<!-- cycle: 257 -->

Fix the two MEDIUM defects surfaced by the cycle-256 audit: (M1) cap `EVOLVE_SCROLLBACK_LINES` at a safe maximum, and (M2) size-limit the `token-usage.json` read path to prevent memory exhaustion.

## Acceptance Criteria

### 1. EVOLVE_SCROLLBACK_LINES clamped to 100 000 max [code]

```bash
# Command
cd go && go test ./internal/bridge/... -run TestScrollbackLines -v 2>&1 | tail -20
# Expected: PASS with "override 2000 → final capture uses 2000" still passing;
#           new subtest "oversized value clamped to 100000" also PASS
```

### 2. Negative case: value above cap is rejected silently [code]

```bash
cd go && go test ./internal/bridge/... -run TestScrollbackLines/oversized -v 2>&1
# Expected: test exits 0; a CapturePane call with 1_000_000_000 must NOT appear
# in fakeTmux.captureScrollback; the recorded value must be <= 100_000
```

### 3. token-usage.json size-limited read — large file returns 0 [code]

```bash
cd go && go test ./internal/bridge/... -run TestBuildReport_TokenUsage/oversized 2>&1
# Expected: test exits 0; a 10 MB token-usage.json produces TokenUsage=0 (not OOM)
```

### 4. Normal token-usage.json still reads correctly [code]

```bash
cd go && go test ./internal/bridge/... -run TestBuildReport_TokenUsage -v 2>&1 | tail -15
# Expected: all existing subtests (valid=7400, missing=0, malformed=0) PASS
```

### 5. Full bridge test suite regression-free [code]

```bash
cd go && go test ./internal/bridge/... 2>&1 | tail -5
# Expected: ok github.com/mickeyyaya/evolve-loop/go/internal/bridge
```
