# Eval: cleanup-sweep

## Code Graders (bash commands that must exit 0)

- `[code]` `test -f .evolve/phases/cleanup-sweep/phase.json`
- `[code]` `test -f .evolve/phases/cleanup-sweep/agent.md`
- `[code]` `evolve phases validate cleanup-sweep 2>&1 | grep -q "OK\|valid\|pass" || (evolve phases list 2>&1 | grep -q "cleanup-sweep")`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/cleanup-sweep/phase.json')); assert d['name']=='cleanup-sweep'; assert d['archetype']=='evaluate'; assert d.get('optional',False)==True"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/cleanup-sweep/phase.json')); sigs=d['outputs']['signals']; assert 'deadcode.symbols' in sigs"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/cleanup-sweep/phase.json')); assert d.get('writes_source',True)==False"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/cleanup-sweep/phase.json')); iw=d['routing']['insert_when']; assert any(r.get('field','') in ('scout.goal_type','build.files_touched') for r in iw)"`

## Regression Evals

- `[code]` `cd go && go test ./internal/phases/... 2>&1 | tail -15 | grep -v "FAIL"`

## Acceptance Checks

- `[code]` `grep -q "deadcode.symbols" .evolve/phases/cleanup-sweep/phase.json`
- `[code]` `grep -qi "deadcode\|unused\|go mod tidy" .evolve/phases/cleanup-sweep/agent.md`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/cleanup-sweep/phase.json')); assert d.get('writes_source',True)==False"` 

## Negative Cases

- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/cleanup-sweep/phase.json')); assert 'writes_source' not in d or d['writes_source']==False"  # must NOT write source — detection only`

## Thresholds
- All checks: pass@1 = 1.0
