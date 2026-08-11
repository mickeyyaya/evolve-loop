# Eval: rollback-plan

## Code Graders (bash commands that must exit 0)

- `[code]` `test -f .evolve/phases/rollback-plan/phase.json`
- `[code]` `test -f .evolve/phases/rollback-plan/agent.md`
- `[code]` `evolve phases validate rollback-plan 2>&1 | grep -q "OK\|valid\|pass" || (evolve phases list 2>&1 | grep -q "rollback-plan")`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/rollback-plan/phase.json')); assert d['name']=='rollback-plan'; assert d['archetype']=='control'; assert d.get('optional',False)==True"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/rollback-plan/phase.json')); sig=d['outputs']['signals']; assert 'rollback.ready' in sig"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/rollback-plan/phase.json')); fi=d['classify']['fail_if_signal']; assert 'rollback.ready' in fi"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/rollback-plan/phase.json')); iw=d['routing']['insert_when'][0]; assert iw['field'] in ('ship.class','scout.risk_level','build.files_touched')"`

## Regression Evals

- `[code]` `cd go && go test ./internal/phases/... 2>&1 | grep -v "^---" | tail -15 | grep -v "FAIL"`

## Acceptance Checks

- `[code]` `grep -q "rollback.ready" .evolve/phases/rollback-plan/phase.json`
- `[code]` `grep -q "fail_if_signal" .evolve/phases/rollback-plan/phase.json`
- `[code]` `grep -qi "revert\|rollback\|blast radius\|known.good" .evolve/phases/rollback-plan/agent.md`

## Thresholds
- All checks: pass@1 = 1.0
