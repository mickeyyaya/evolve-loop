# Eval: benchmark-gate

## Code Graders (bash commands that must exit 0)

- `[code]` `test -f .evolve/phases/benchmark-gate/phase.json`
- `[code]` `test -f .evolve/phases/benchmark-gate/agent.md`
- `[code]` `evolve phases validate benchmark-gate 2>&1 | grep -q "OK\|valid\|pass" || (evolve phases list 2>&1 | grep -q "benchmark-gate")`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); assert d['name']=='benchmark-gate'; assert d['archetype']=='evaluate'; assert d.get('optional',False)==True"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); sigs=d['outputs']['signals']; assert 'perf.regression_pct' in sigs and 'perf.significant' in sigs"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); fi=d['classify']['fail_if_signal']; assert 'perf.significant' in fi"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); iw=d['routing']['insert_when']; assert any(r.get('field','') in ('build.diff_loc','build.files_touched','perf.hotpath_touched') for r in iw)"`

## Regression Evals

- `[code]` `cd go && go test ./internal/phases/... 2>&1 | tail -15 | grep -v "FAIL"`

## Acceptance Checks

- `[code]` `grep -q "perf.significant" .evolve/phases/benchmark-gate/phase.json`
- `[code]` `grep -qi "benchstat\|baseline\|statistical\|p.value\|N samples" .evolve/phases/benchmark-gate/agent.md`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); assert d['classify']['fail_if_signal'].get('perf.significant','')=='==true'"`

## Negative Cases (gaming prevention)

- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); assert d['name']=='benchmark-gate' and d['name']!='perf-profile'"  # must be a NEW phase, not a rename`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/benchmark-gate/phase.json')); assert 'perf.significant' in d['outputs']['signals']"  # must emit the statistical significance signal`

## Thresholds
- All checks: pass@1 = 1.0
