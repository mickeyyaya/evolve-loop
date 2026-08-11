# Eval: fuzz-probe

## Code Graders (bash commands that must exit 0)

- `[code]` `test -f .evolve/phases/fuzz-probe/phase.json`
- `[code]` `test -f .evolve/phases/fuzz-probe/agent.md`
- `[code]` `test -f agents/evolve-fuzz-probe.md`
- `[code]` `cd go && evolve phases validate fuzz-probe 2>&1 | grep -q "OK\|valid\|pass" || (evolve phases list 2>&1 | grep -q "fuzz-probe")`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); assert d['name']=='fuzz-probe'; assert d['archetype']=='evaluate'; assert d.get('optional',False)==True"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); sigs=d['outputs']['signals']; assert 'fuzz.crashers' in sigs and 'fuzz.coverage_new' in sigs"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); fi=d['classify']['fail_if_signal']; assert 'fuzz.crashers' in fi"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); iw=d['routing']['insert_when']; fields=[r.get('field','') for r in iw]; assert len(iw)>=1"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); assert d.get('writes_source',True)==False"`

## Regression Evals

- `[code]` `cd go && go test ./internal/phases/... 2>&1 | tail -15 | grep -v "^FAIL"`

## Acceptance Checks

- `[code]` `grep -q "fuzz.crashers" .evolve/phases/fuzz-probe/phase.json`
- `[code]` `grep -qi "fuzz\|go test -fuzz\|fuzztime\|corpus\|crasher" .evolve/phases/fuzz-probe/agent.md`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); threshold=d['classify']['fail_if_signal'].get('fuzz.crashers',''); assert '>0' in threshold, f'expected >0 threshold for fuzz.crashers, got: {threshold}'"`

## Negative Cases (gaming prevention)

- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); assert d['name']=='fuzz-probe'"  # must be distinct name`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); assert d.get('writes_source',True)==False"  # detection only`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); assert 'fuzz.crashers' in d['outputs']['signals']"  # must emit crasher count`

## Edge Cases

- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); assert 'routing' in d and 'insert_when' in d.get('routing',{}), 'routing.insert_when required'"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/fuzz-probe/phase.json')); assert d.get('kind','llm')=='llm', 'must be kind:llm'"`
- `[code]` `grep -qi "parser\|decode\|unmarshal\|security\|input.handling" .evolve/phases/fuzz-probe/agent.md  # agent must scope to appropriate surfaces`

## Thresholds
- All checks: pass@1 = 1.0
