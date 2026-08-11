# Eval: implement-security-scan-phase

## Goal
Verify that the security-scan user phase spec is correct, loadable, and properly routed.

## Acceptance Criteria

### AC1: Phase JSON is valid and loads without error [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  test -f .evolve/phases/security-scan/phase.json && \
  cat .evolve/phases/security-scan/phase.json | python3 -c "import json,sys; d=json.load(sys.stdin); assert d.get('optional') == True, 'must be optional'; assert d.get('name') == 'security-scan'; print('PASS')"
```
Expected: `PASS`

### AC2: Phase passes ValidateUserSpec (go test) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  go test ./internal/phasespec/... -run TestApplyUserRouting -v 2>&1 | grep -E "PASS|FAIL|ok" | head -5
```
Expected: contains `ok` or `PASS` with no `FAIL`

### AC3: Phase appears in evolve phases list [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  ./evolve phases list 2>/dev/null | grep "security-scan"
```
Expected: output contains `security-scan`

### AC4: Phase spec includes security.severity_max signal output [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  python3 -c "
import json
with open('.evolve/phases/security-scan/phase.json') as f:
    d = json.load(f)
sigs = d.get('outputs', {}).get('signals', [])
assert 'security.severity_max' in sigs, f'security.severity_max not in signals: {sigs}'
print('PASS')
"
```
Expected: `PASS`

### AC5: Phase routing trigger is set (insert_when build.files_touched > 0) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  python3 -c "
import json
with open('.evolve/phases/security-scan/phase.json') as f:
    d = json.load(f)
routing = d.get('routing', {})
iw = routing.get('insert_when', [])
assert len(iw) > 0, 'no insert_when triggers'
fields = [c.get('field','') for c in iw]
assert any('build' in f or 'files' in f for f in fields), f'no build-related trigger: {fields}'
print('PASS')
"
```
Expected: `PASS`

### AC6: Negative — phase without optional:true is rejected by ValidateUserSpec [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  go test ./internal/phasespec/... -run TestApplyUserRouting_SkipsInvalid -v 2>&1 | grep -E "PASS|ok"
```
Expected: contains `PASS` or `ok` (existing test proves non-optional phases are rejected)

### AC7: Edge — phase with empty name is rejected [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  go test ./internal/phasespec/... -run TestDiscoverUserSpecs -v 2>&1 | grep -E "PASS|ok"
```
Expected: contains `PASS` or `ok`
