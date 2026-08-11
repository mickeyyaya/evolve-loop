# Eval: mutation-gate-phase

Verifies that the `mutation-gate` user phase is fully materialized: descriptor, agent persona (canonical + bridge), and validation green.

## AC1: Phase descriptor exists [code]

```bash
test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/phases/mutation-gate/phase.json && echo "PASS" || echo "FAIL"
```

Expected: `PASS`

## AC2: Canonical agent persona exists [code]

```bash
test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/phases/mutation-gate/agent.md && echo "PASS" || echo "FAIL"
```

Expected: `PASS`

## AC3: Bridge agent persona exists [code]

```bash
test -f /Users/danleemh/ai/claude/evolve-loop/agents/evolve-mutation-gate.md && echo "PASS" || echo "FAIL"
```

Expected: `PASS`

## AC4: Phase validates with evolve CLI [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop && go/evolve phases validate mutation-gate 2>&1; echo "EXIT:$?"
```

Expected output contains `EXIT:0`; no error lines about missing fields.

## AC5: Phase appears in phases list [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop && go/evolve phases list 2>&1 | grep "mutation-gate"
```

Expected: a line containing `mutation-gate` with `user` source and `true` optional flag.

## AC6: phase.json has required gate fields [code]

```bash
python3 -c "
import json, sys
d = json.load(open('/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/mutation-gate/phase.json'))
assert d['name'] == 'mutation-gate', 'name'
assert d['kind'] == 'llm', 'kind'
assert d['optional'] == True, 'optional'
assert d['archetype'] == 'evaluate', 'archetype'
assert 'mutation.score' in d['outputs']['signals'], 'score signal'
assert 'mutation.survivors' in d['outputs']['signals'], 'survivors signal'
assert 'fail_if_signal' in d['classify'], 'fail_if_signal gate missing'
assert 'mutation.score' in d['classify']['fail_if_signal'], 'score gate'
print('PASS')
"
```

Expected: `PASS`

## AC7: Agent persona instructs mutation tool use [code]

```bash
grep -qi "gremlins\|go-mutesting\|mutation" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-mutation-gate.md && echo "PASS" || echo "FAIL"
```

Expected: `PASS`

## Negative case: validation rejects a phase.json missing fail_if_signal [model]

If `classify.fail_if_signal` is removed from the phase.json, `evolve phases validate mutation-gate` should report a validation error (or the phase would silently miss its gate semantics). The presence of `fail_if_signal` is the distinguishing property of a gate phase.
