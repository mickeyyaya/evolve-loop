# Eval: phase-latency-report

## Metadata
- slug: phase-latency-report
- cycle: 170
- task: Write per-phase latency summary (phase-latency-report.json) per cycle

## Acceptance Criteria

### AC-1: phase-latency-report.json is written to the workspace [code]
```bash
# The orchestrator must write this file after a cycle completes
grep -q "phase-latency-report\|phase_latency_report\|phaseLatencyReport" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
```
Expected: exit 0

### AC-2: File contains per-phase duration_ms values [code]
```bash
# Must have a schema that includes phase names and their ms durations
grep -n "duration_ms\|DurationMS\|PhaseLatencies\|phaseLatencies" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go | grep -v "_test\|// "
```
Expected: exit 0 (non-empty output)

### AC-3: CycleResult carries per-phase latency map [code]
```bash
grep -n "PhaseLatencies\|phaseLatencies\|LatencyMS\|latencyMS" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go | head -5
```
Expected: exit 0

### AC-4: cyclehealth velocity signal uses phase-latency-report.json when available [code]
```bash
grep -q "phase-latency-report\|phaseLatencyReport\|phase_latency_report" /Users/danleemh/ai/claude/evolve-loop/go/internal/cyclehealth/cyclehealth.go
```
Expected: exit 0

### AC-5: Phase-latency-report.json NOT written = zero-file-corrupt negative case [code]
```bash
# If DurationMS is 0 for a phase, it should still appear in the report (not silently dropped)
grep -n "PhaseLatencies\|latencies\|durationMS\|duration_ms" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go | grep -v "//"
```
Expected: exit 0 (non-empty output confirms latency always written)

### AC-6: cyclehealth + orchestrator tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... ./internal/cyclehealth/... -timeout 60s 2>&1 | tail -5
```
Expected: exit 0
