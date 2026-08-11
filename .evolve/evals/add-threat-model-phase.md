# Eval: add-threat-model-phase

## Purpose
Verify that the threat-model phase is fully designed and wired into the pipeline with a concrete persona, phase-registry entry, and supporting research documentation.

## Acceptance Criteria

### AC-1: phase-registry.json contains threat-model entry [code]
```bash
python3 -c "
import json, sys
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
names = [p['name'] for p in d['phases']]
assert 'threat-model' in names, f'threat-model not in registry. Found: {names}'
print('PASS: threat-model present in phase-registry.json')
"
```

### AC-2: threat-model phase has required fields [code]
```bash
python3 -c "
import json
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
phase = next(p for p in d['phases'] if p['name'] == 'threat-model')
required = ['name', 'role', 'optional', 'inputs', 'outputs']
for field in required:
    assert field in phase, f'Missing field: {field}'
assert phase.get('optional') == True, 'threat-model must be optional (not mandatory spine)'
print('PASS: threat-model phase has all required fields and is optional')
"
```

### AC-3: agent persona file exists [code]
```bash
test -f agents/evolve-threat-model.md && echo 'PASS: agents/evolve-threat-model.md exists' || (echo 'FAIL: agents/evolve-threat-model.md missing'; exit 1)
```

### AC-4: persona file has substantial content (inputs, outputs, threat categories) [code]
```bash
wc -l agents/evolve-threat-model.md | awk '{if ($1 >= 40) print "PASS: persona has " $1 " lines (>= 40)"; else {print "FAIL: persona too short: " $1 " lines"; exit 1}}'
```

### AC-5: research document exists with cited sources [code]
```bash
ls knowledge-base/research/threat-model-phase-design-2026-06.md 2>/dev/null && echo 'PASS: research doc exists' || (echo 'FAIL: research doc missing'; exit 1)
```

### AC-6: research doc contains gap analysis and citations [code]
```bash
python3 -c "
import re
with open('knowledge-base/research/threat-model-phase-design-2026-06.md') as f:
    content = f.read()
checks = [
    ('Gap analysis', r'[Gg]ap [Aa]nalysis|partially covered|genuinely absent'),
    ('Citations', r'OWASP|ASTRIDE|SSDLC|MITRE|arXiv|https://'),
    ('Position in pipeline', r'[Pp]osition|gate_in|gate_out|after triage|before tdd|before build'),
    ('Negative case coverage', r'[Nn]egative case|[Ee]dge case|reject|skip when'),
]
for label, pattern in checks:
    m = re.search(pattern, content)
    assert m, f'Missing in research doc: {label} (pattern: {pattern})'
print('PASS: research doc has gap analysis, citations, pipeline position, and edge cases')
"
```

### AC-7 (negative): threat-model is NOT mandatory spine [code]
```bash
python3 -c "
import json
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
mandatory = d.get('config', {}).get('mandatory_phases', [])
assert 'threat-model' not in mandatory, f'FAIL: threat-model must not be in mandatory_phases: {mandatory}'
print('PASS: threat-model is not in mandatory_phases (correctly optional)')
"
```

### AC-8 (edge): phase-registry.json is valid JSON after changes [code]
```bash
python3 -c "import json; json.load(open('docs/architecture/phase-registry.json')); print('PASS: phase-registry.json is valid JSON')"
```
