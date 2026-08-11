# Eval: builder-phase-usage-sidecar

**Cycle:** 189
**Task:** After each phase records a timing entry in the orchestrator, also write a
`<workspace>/<phase>-usage.json` sidecar containing `{phase, cost_usd, duration_ms,
attempt_count, verdict}`. The auditor has flagged POSTHOC cost/duration as "pending"
for two consecutive cycles because this file is absent.

---

## Criteria

### AC-1: Per-phase sidecar write logic exists in orchestrator [code]

```bash
grep -n "usage.json\|usage-sidecar\|writeUsageSidecar\|phaseUsage\|usageFile" go/internal/core/orchestrator.go
```

Expected: ≥1 match showing the sidecar write path.

---

### AC-2: Sidecar file contains required JSON fields [code]

```bash
# The struct or JSON marshal must include cost_usd, duration_ms, attempt_count, verdict
grep -n "cost_usd\|CostUSD\|duration_ms\|DurationMs\|attempt_count\|AttemptCount" go/internal/core/orchestrator.go | grep -v "phaseTimingEntry\|phase-timing.json" | head -10
```

Expected: ≥1 match showing a new struct or marshal block for the per-phase sidecar.

---

### AC-3: Sidecar is written for the build phase (at minimum) [code]

```bash
# The build phase timing entry must trigger a sidecar write
grep -n "build.*usage\|usage.*build\|PhasesBuild\|\"build\".*usage" go/internal/core/orchestrator.go | head -5
```

Expected: ≥1 match showing build-specific wiring OR a generic per-phase write path that includes build.

---

### AC-4: Unit test verifies sidecar file exists after a cycle run [code]

```bash
grep -n "usage.json\|UsageSidecar\|usageSidecar\|usage.*sidecar" go/internal/core/ -r | grep "_test.go" | head -10
```

Expected: ≥1 match in a test file.

---

### AC-5: Phase-timing-and-diagnostics doc updated [code]

```bash
grep -n "usage.json\|usage.*sidecar\|phase.*sidecar\|POSTHOC\|posthoc" docs/architecture/phase-timing-and-diagnostics.md
```

Expected: ≥1 match (the doc must describe the new per-phase sidecar).

---

### AC-6: Tests pass [code]

```bash
cd go && go test ./internal/core/... -run "TestOrchestrator.*Usage\|TestPhase.*Usage\|TestUsageSidecar\|TestWritesPhase" -count=1 2>&1 | tail -5
```

Expected: output contains "ok" with no FAIL lines.

---

## Negative Cases

### NC-1: Sidecar write does NOT break existing phase-timing.json [code]

```bash
grep -n "phase-timing.json\|phase-timing\|phaseTimingEntry" go/internal/core/orchestrator.go | head -10
```

Expected: the phase-timing.json write path remains unchanged (still present).

```bash
cd go && go test ./internal/core/... -run "TestOrchestrator_Timing\|TestPhase_Timing" -count=1 2>&1 | tail -5
```

Expected: "ok" with no FAIL lines (existing timing tests still pass).

### NC-2: Sidecar write failure is non-fatal (WARN only) [code]

```bash
grep -n "WARN.*usage\|usage.*WARN\|Stderr.*usage\|usage.*Stderr" go/internal/core/orchestrator.go | head -5
```

Expected: ≥1 match showing the sidecar write failure emits a warning but does not abort the cycle.
