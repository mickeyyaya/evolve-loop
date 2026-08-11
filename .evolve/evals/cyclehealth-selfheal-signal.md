# Eval: cyclehealth-selfheal-signal

## Metadata
- slug: cyclehealth-selfheal-signal
- cycle: 186
- task: Add the `self_heal_events` signal (13th) to cyclehealth — scan the ledger for kind=phase_retry / kind=backfill entries in the current cycle and surface ONE WARN anomaly per recovery event so self-heal activation is visible in cycle-health.json.

> Pins the cyclehealth signal that turns the orchestrator's self-heal trail
> (phase_retry / backfill ledger entries) into an operator-visible WARN anomaly.
> WARN (never fatal) so a recovered-but-healthy cycle is not blocked (hypothesis H1).
> The anti-no-op evidence (AC-4) is the load-bearing one: a signal that always fired,
> or grepped a magic string, would FAIL it. Source: cycle-186 (re-implements cycle-184/185's
> unshipped work; cycle-184 FAILed audit on mixed ACS predicate — code was sound; cycle-185
> reset before scout completed); scout-report.md Task 1.

## Acceptance Criteria

### AC-1: self_heal_events is the 13th signal in signalNames() [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/cyclehealth/... -run '^TestSignalNames_ReturnsThirteen$' -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestSignalNames_ReturnsThirteen` (no "no tests to run" line)

### AC-2: kind=phase_retry surfaces a self_heal_events WARN anomaly (phase + exit code, not fatal) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/cyclehealth/... -run '^TestCheck_SelfHealEvents_PhaseRetry_Warn$' -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestCheck_SelfHealEvents_PhaseRetry_Warn`

### AC-3: kind=backfill surfaces a self_heal_events WARN anomaly naming the phase [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/cyclehealth/... -run '^TestCheck_SelfHealEvents_Backfill_Warn$' -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestCheck_SelfHealEvents_Backfill_Warn`

### AC-4: a clean cycle (no retry/backfill) emits ZERO self_heal_events anomalies [code]
Anti-no-op: the signal must key off real ledger events, not always fire. Bundled
with the one-per-event count and cross-cycle isolation assertions.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/cyclehealth/... -run '^TestCheck_SelfHealEvents_(NoEvents_NoAnomaly|OnePerEvent|OtherCycle_Ignored)$' -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: three `--- PASS:` lines (NoEvents_NoAnomaly, OnePerEvent, OtherCycle_Ignored); no FAIL

### AC-5: phase-timing-and-diagnostics.md documents the self_heal_events signal [code]
```bash
grep -c "self_heal_events" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-timing-and-diagnostics.md
```
Expected: at least 1

### AC-6: cyclehealth package still passes in full (no regression) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/cyclehealth/... -count=1 -timeout 60s 2>&1 | grep -E "^ok |^FAIL "
```
Expected: lines starting with `ok` only (no FAIL)

## Negative Cases

### NC-1: a self-heal entry tagged for a DIFFERENT cycle must not leak into this cycle's report [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/cyclehealth/... -run '^TestCheck_SelfHealEvents_OtherCycle_Ignored$' -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestCheck_SelfHealEvents_OtherCycle_Ignored`
