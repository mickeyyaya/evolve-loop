# Eval: wave1-security-test-phases

## Acceptance Criteria

### AC-1: threat-model phase.json exists [code]
```bash
test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/phases/threat-model/phase.json
# Expected: exit 0
```

### AC-2: test-amplification phase.json exists [code]
```bash
test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/phases/test-amplification/phase.json
# Expected: exit 0
```

### AC-3: threat-model agent.md exists [code]
```bash
test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/phases/threat-model/agent.md
# Expected: exit 0
```

### AC-4: test-amplification agent.md exists [code]
```bash
test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/phases/test-amplification/agent.md
# Expected: exit 0
```

### AC-5: Both phases validate green [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && evolve phases validate threat-model && evolve phases validate test-amplification
# Expected: exit 0 (both)
```

### AC-6: threat-model has CRITICAL severity gate [code]
```bash
python3 -c "
import json
d = json.load(open('/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/threat-model/phase.json'))
fis = d['classify']['fail_if_signal']
# Must block on unmitigated CRITICAL threats
assert any('threat.severity_max' in k or 'CRITICAL' in str(fis) for k in fis), 'missing CRITICAL threat gate'
print('OK')
"
# Expected: OK
```

### AC-7: threat-model requires Threats and Mitigations sections [code]
```bash
python3 -c "
import json
d = json.load(open('/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/threat-model/phase.json'))
secs = d['classify'].get('require_sections', [])
assert 'Threats' in secs and 'Mitigations' in secs, 'threat-model must require Threats + Mitigations sections'
print('OK')
"
# Expected: OK
```

### AC-8: test-amplification routing gates on non-trivial cycles [code]
```bash
python3 -c "
import json
d = json.load(open('/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/test-amplification/phase.json'))
rules = d['routing']['insert_when']
fields = [r.get('field','') for r in rules]
assert any('cycle_size' in f or 'files_touched' in f for f in fields), 'test-amplification must gate on cycle size or files touched'
print('OK')
"
# Expected: OK
```

### AC-9: test-amplification emits amplify.tests_added signal [code]
```bash
python3 -c "
import json
d = json.load(open('/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/test-amplification/phase.json'))
sigs = d.get('outputs', {}).get('signals', [])
assert any('amplify.' in s for s in sigs), 'test-amplification must emit amplify.* signals'
print('OK')
"
# Expected: OK
```

### NEGATIVE: test-amplification must be optional (not mandatory) [code]
```bash
python3 -c "
import json
d = json.load(open('/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/test-amplification/phase.json'))
assert d.get('optional') == True, 'test-amplification must be optional:true per ADR-0035'
print('OK')
"
# Expected: OK
```

### NEGATIVE: threat-model must be plan archetype (not evaluate) [code]
```bash
python3 -c "
import json
d = json.load(open('/Users/danleemh/ai/claude/evolve-loop/.evolve/phases/threat-model/phase.json'))
assert d.get('archetype') == 'plan', 'threat-model is a plan phase (scans future surfaces, not built code)'
print('OK')
"
# Expected: OK
```
