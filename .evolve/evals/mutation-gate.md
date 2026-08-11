# Eval: mutation-gate

## Code Graders (bash commands that must exit 0)

- `[code]` `test -f .evolve/phases/mutation-gate/phase.json`
- `[code]` `test -f .evolve/phases/mutation-gate/agent.md`
- `[code]` `test -f agents/evolve-mutation-gate.md`
- `[code]` `cd go && evolve phases validate mutation-gate 2>&1 | grep -q "OK\|valid\|pass" || (evolve phases list 2>&1 | grep -q "mutation-gate")`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); assert d['name']=='mutation-gate'; assert d['archetype']=='evaluate'; assert d.get('optional',False)==True"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); sigs=d['outputs']['signals']; assert 'mutation.score' in sigs and 'mutation.survivors' in sigs"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); fi=d['classify']['fail_if_signal']; assert 'mutation.score' in fi"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); iw=d['routing']['insert_when']; fields=[r.get('field','') for r in iw]; assert any('test' in f or 'amplify' in f for f in fields)"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); assert d.get('writes_source',True)==False"`

## Regression Evals

- `[code]` `cd go && go test ./internal/phases/... 2>&1 | tail -15 | grep -v "^FAIL"`

## Acceptance Checks

- `[code]` `grep -q "mutation.score" .evolve/phases/mutation-gate/phase.json`
- `[code]` `grep -q "mutation.survivors" .evolve/phases/mutation-gate/phase.json`
- `[code]` `grep -qi "mutation\|mutant\|diff.scoped\|go-mutesting\|coverage" .evolve/phases/mutation-gate/agent.md`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); threshold=d['classify']['fail_if_signal'].get('mutation.score',''); assert '<60' in threshold or threshold.startswith('<'), f'unexpected threshold: {threshold}'"`

## Negative Cases (gaming prevention)

- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); assert d['name']=='mutation-gate'"  # must be distinct name`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); assert d.get('writes_source',True)==False"  # must not write source`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); assert 'mutation.score' in d['outputs']['signals']"  # signal must be declared, not implicit`

## Edge Cases

- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); assert 'routing' in d and 'insert_when' in d.get('routing',{}), 'routing.insert_when required'"`
- `[code]` `python3 -c "import json; d=json.load(open('.evolve/phases/mutation-gate/phase.json')); assert d.get('kind','llm')=='llm', 'must be kind:llm (LLM-orchestrated tool run)"`

## Thresholds
- All checks: pass@1 = 1.0
