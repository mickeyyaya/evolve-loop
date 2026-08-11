# Eval: wave2-restore

Verify that all 3 lost Wave 2 micro-phases (rollback-plan, cleanup-sweep, benchmark-gate) are correctly
implemented on main — these were built in cycle 220 worktree but lost when the cycle reset before ship.
Also verifies bridge agent files in agents/ exist for dispatch compatibility.

## Code Graders (bash commands that must exit 0)

### Phase directories exist

- `[code]` `test -f .evolve/phases/rollback-plan/phase.json && test -f .evolve/phases/rollback-plan/agent.md`
- `[code]` `test -f .evolve/phases/cleanup-sweep/phase.json && test -f .evolve/phases/cleanup-sweep/agent.md`
- `[code]` `test -f .evolve/phases/benchmark-gate/phase.json && test -f .evolve/phases/benchmark-gate/agent.md`

### Bridge agent files exist

- `[code]` `test -f agents/evolve-rollback-plan.md`
- `[code]` `test -f agents/evolve-cleanup-sweep.md`
- `[code]` `test -f agents/evolve-benchmark-gate.md`

### Validate each phase

- `[code]` `cd go && evolve phases validate rollback-plan 2>&1 | grep -q "OK\|valid\|pass" || (evolve phases list 2>&1 | grep -q "rollback-plan")`
- `[code]` `cd go && evolve phases validate cleanup-sweep 2>&1 | grep -q "OK\|valid\|pass" || (evolve phases list 2>&1 | grep -q "cleanup-sweep")`
- `[code]` `cd go && evolve phases validate benchmark-gate 2>&1 | grep -q "OK\|valid\|pass" || (evolve phases list 2>&1 | grep -q "benchmark-gate")`

### Structural correctness — rollback-plan

- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/rollback-plan/phase.json')); assert d['name']=='rollback-plan' and d['archetype']=='control' and d.get('optional')==True"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/rollback-plan/phase.json')); assert 'rollback.ready' in d['outputs']['signals']"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/rollback-plan/phase.json')); assert 'rollback.ready' in d['classify']['fail_if_signal']"`

### Structural correctness — cleanup-sweep

- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/cleanup-sweep/phase.json')); assert d['name']=='cleanup-sweep' and d['archetype']=='evaluate' and d.get('optional')==True"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/cleanup-sweep/phase.json')); assert 'deadcode.symbols' in d['outputs']['signals']"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/cleanup-sweep/phase.json')); assert d.get('writes_source',True)==False"`

### Structural correctness — benchmark-gate

- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); assert d['name']=='benchmark-gate' and d['archetype']=='evaluate' and d.get('optional')==True"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); sigs=d['outputs']['signals']; assert 'perf.regression_pct' in sigs and 'perf.significant' in sigs"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); assert 'perf.significant' in d['classify']['fail_if_signal']"`

## Regression Evals

- `[code]` `cd go && go test ./internal/phases/... 2>&1 | tail -15 | grep -v "^FAIL"`

## Negative Cases (gaming prevention)

- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/cleanup-sweep/phase.json')); assert d.get('writes_source',True)==False"  # cleanup-sweep must not write source`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); assert d['name']!='perf-profile'"  # must be a separate phase, not a rename`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/rollback-plan/phase.json')); assert 'fail_if_signal' in d.get('classify',{})"  # rollback.ready must be a hard gate`

## Thresholds
- All checks: pass@1 = 1.0
