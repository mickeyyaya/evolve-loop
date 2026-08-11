# Eval: phases-quality-gates-wave2

## Task
Ship Wave 2 quality-gate phases as config-only phase descriptors (zero Go):
- `benchmark-gate` — statistical perf regression gate
- `fuzz-probe` — Go-native fuzz harness probe
- `cleanup-sweep` — dead-code / unused-dep detection
- `rollback-plan` — pre-ship revert-readiness check
(mutation-gate already exists at .evolve/phases/mutation-gate/; include validate-green fix if needed)

## Acceptance Criteria

### AC1 — all 5 Wave 2 phase directories exist [code]
```bash
for p in mutation-gate benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do
  if [ -d "/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/$p" ]; then
    echo "OK: $p"
  else
    echo "MISSING: $p"
  fi
done
```
Expected: All 5 lines show `OK:`.

### AC2 — each phase has required files (phase.json + agent.md) [code]
```bash
for p in mutation-gate benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do
  dir="/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/$p"
  [ -f "$dir/phase.json" ] && [ -f "$dir/agent.md" ] && echo "OK: $p" || echo "MISSING-FILES: $p"
done
```
Expected: All 5 lines show `OK:`.

### AC3 — phase.json required fields present in each phase [code]
```bash
for p in mutation-gate benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do
  f="/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/$p/phase.json"
  python3 -c "
import json,sys
d=json.load(open('$f'))
required=['name','kind','optional','archetype','outputs']
missing=[k for k in required if k not in d]
print('MISSING' if missing else 'OK', '$p', missing)
"
done
```
Expected: All 5 lines show `OK`.

### AC4 — each phase.json has valid JSON syntax [code]
```bash
for p in mutation-gate benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do
  f="/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/$p/phase.json"
  python3 -c "import json; json.load(open('$f')); print('OK: $p')" 2>&1
done
```
Expected: All 5 lines show `OK:`.

### AC5 — two-tier naming: no single-word new phase names [code]
```bash
for p in benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do
  echo "$p" | grep -qE "^[a-z]+-[a-z]" && echo "OK-naming: $p" || echo "BAD-naming: $p"
done
```
Expected: All 4 new phases show `OK-naming:` (mutation-gate already existed, exempt from new check).

### AC6 — negative: existing phase catalog not broken by additions [code]
```bash
for p in fault-localization bug-reproduction smell-scan threat-model test-amplification; do
  [ -f "/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/$p/phase.json" ] && echo "INTACT: $p" || echo "BROKEN: $p"
done
```
Expected: All 5 existing Wave 1 phases show `INTACT:`.

### AC7 — each phase has at least one output signal [code]
```bash
for p in mutation-gate benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do
  f="/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/$p/phase.json"
  python3 -c "
import json
d=json.load(open('$f'))
sigs=d.get('outputs',{}).get('signals',[])
print('OK-signals' if sigs else 'NO-SIGNALS', '$p', sigs)
"
done
```
Expected: All 5 phases show `OK-signals`.

### AC8 — rollback-plan fail_if_signal on rollback.ready==false [model]
Review `.evolve/phases/rollback-plan/phase.json` to confirm `classify.fail_if_signal` contains `{"rollback.ready": "==false"}` per the micro-phase-catalog spec. This is the safety-critical gate that blocks a ship if rollback is not ready.
