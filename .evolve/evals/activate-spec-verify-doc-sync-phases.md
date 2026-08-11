# Eval: activate-spec-verify-doc-sync-phases

## Task
Add `spec-verify` and `doc-sync` to `conditional_mandatory` in
`docs/architecture/phase-registry.json` so they run automatically on non-trivial cycles,
and add Go config tests verifying the new entries parse and are recognized.

## Acceptance Criteria

### C1: phase-registry.json conditional_mandatory includes spec-verify [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
python3 -c "
import json, sys
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
cm = d.get('config', {}).get('conditional_mandatory', {})
if 'spec-verify' in cm:
    print('PASS: spec-verify in conditional_mandatory:', cm['spec-verify'])
else:
    print('FAIL: spec-verify missing from conditional_mandatory. Found:', list(cm.keys()))
    sys.exit(1)
"
```

### C2: phase-registry.json conditional_mandatory includes doc-sync [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
python3 -c "
import json, sys
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
cm = d.get('config', {}).get('conditional_mandatory', {})
if 'doc-sync' in cm:
    print('PASS: doc-sync in conditional_mandatory:', cm['doc-sync'])
else:
    print('FAIL: doc-sync missing from conditional_mandatory. Found:', list(cm.keys()))
    sys.exit(1)
"
```

### C3: conditional_mandatory expressions use valid cycle_size syntax [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
python3 -c "
import json, sys
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
cm = d.get('config', {}).get('conditional_mandatory', {})
valid_ops = ['!=', '==', '>=', '<=', '>', '<']
errors = []
for phase, expr in cm.items():
    if not any(op in expr for op in valid_ops):
        errors.append(f'{phase}: {expr!r} has no valid comparison operator')
if errors:
    print('FAIL:', errors)
    sys.exit(1)
print('PASS: all', len(cm), 'conditional_mandatory entries have valid operators')
for k, v in cm.items():
    print(f'  {k}: {v}')
"
```

### C4: Go config tests pass after change [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/config/... -count=1 -timeout 60s 2>&1 | tail -5
```

### C5: Negative — mandatory core phases (scout, build, audit, ship) are not removed [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
python3 -c "
import json, sys
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
mandatory = d.get('config', {}).get('mandatory_phases', [])
required = {'scout', 'build', 'audit', 'ship'}
missing = required - set(mandatory)
if missing:
    print('FAIL: core mandatory phases removed:', missing)
    sys.exit(1)
print('PASS: all core mandatory phases present:', mandatory)
"
```
