# Eval: add-change-impact-phase

## Purpose
Verify that the change-impact analysis phase is designed and registered: phase-registry entry, agent persona, and research documentation with cited sources showing the genuine gap this fills.

## Acceptance Criteria

### AC-1: phase-registry.json contains change-impact entry [code]
```bash
python3 -c "
import json
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
names = [p['name'] for p in d['phases']]
assert 'change-impact' in names, f'change-impact not in registry. Found: {names}'
print('PASS: change-impact present in phase-registry.json')
"
```

### AC-2: change-impact phase is optional and has inputs/outputs [code]
```bash
python3 -c "
import json
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
phase = next(p for p in d['phases'] if p['name'] == 'change-impact')
assert phase.get('optional') == True, 'change-impact must be optional'
assert 'inputs' in phase, 'Missing inputs'
assert 'outputs' in phase, 'Missing outputs'
assert 'files' in phase['outputs'] and len(phase['outputs']['files']) > 0, 'outputs.files must not be empty'
print('PASS: change-impact phase has correct structure')
"
```

### AC-3: agent persona file exists [code]
```bash
test -f agents/evolve-change-impact.md && echo 'PASS: agents/evolve-change-impact.md exists' || (echo 'FAIL: agents/evolve-change-impact.md missing'; exit 1)
```

### AC-4: persona has minimum content (role description, responsibilities, output format) [code]
```bash
wc -l agents/evolve-change-impact.md | awk '{if ($1 >= 30) print "PASS: persona has " $1 " lines (>= 30)"; else {print "FAIL: persona too short: " $1 " lines"; exit 1}}'
```

### AC-5: research document exists [code]
```bash
ls knowledge-base/research/change-impact-analysis-2026-06.md 2>/dev/null && echo 'PASS: research doc exists' || (echo 'FAIL: research doc missing'; exit 1)
```

### AC-6: research doc explains the gap (not covered by existing phases) [code]
```bash
python3 -c "
import re
with open('knowledge-base/research/change-impact-analysis-2026-06.md') as f:
    content = f.read()
checks = [
    ('Gap vs existing phases', r'[Ss]cout|[Tt]riage|genuinely (absent|missing)|not covered'),
    ('External references', r'CodeScene|SonarQube|Understand|change coupling|impact analysis|https://|arXiv'),
    ('Pipeline position', r'[Pp]osition|after scout|before triage|gate_in|gate_out'),
]
for label, pattern in checks:
    m = re.search(pattern, content)
    assert m, f'Missing in research doc: {label}'
print('PASS: research doc explains the gap with citations and pipeline position')
"
```

### AC-7 (negative): change-impact is NOT in mandatory_phases [code]
```bash
python3 -c "
import json
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
mandatory = d.get('config', {}).get('mandatory_phases', [])
assert 'change-impact' not in mandatory, f'FAIL: change-impact should not be mandatory: {mandatory}'
print('PASS: change-impact is not in mandatory_phases')
"
```

### AC-8 (edge): phase-registry.json remains valid JSON [code]
```bash
python3 -c "import json; json.load(open('docs/architecture/phase-registry.json')); print('PASS: phase-registry.json is valid JSON')"
```
