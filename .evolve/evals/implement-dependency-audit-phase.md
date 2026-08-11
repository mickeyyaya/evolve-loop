# Eval: implement-dependency-audit-phase

## Goal
Verify that the dependency-audit user phase spec is correct, loadable, and properly routed to trigger on go.mod changes.

## Acceptance Criteria

### AC1: Phase JSON is valid and loadable [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  test -f .evolve/phases/dependency-audit/phase.json && \
  cat .evolve/phases/dependency-audit/phase.json | python3 -c "import json,sys; d=json.load(sys.stdin); assert d.get('optional') == True; assert d.get('name') == 'dependency-audit'; print('PASS')"
```
Expected: `PASS`

### AC2: Phase appears in evolve phases list [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  ./evolve phases list 2>/dev/null | grep "dependency-audit"
```
Expected: output contains `dependency-audit`

### AC3: Both new phases appear together (catalog integration) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  ./evolve phases list 2>/dev/null | grep -E "security-scan|dependency-audit" | wc -l | tr -d ' '
```
Expected: `2`

### AC4: Phase outputs dependency.severity_max signal [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  python3 -c "
import json
with open('.evolve/phases/dependency-audit/phase.json') as f:
    d = json.load(f)
sigs = d.get('outputs', {}).get('signals', [])
assert any('dependency' in s or 'dep' in s for s in sigs), f'no dependency signal: {sigs}'
print('PASS')
"
```
Expected: `PASS`

### AC5: Phase classify rules defined (fail_if_signal or require_sections) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  python3 -c "
import json
with open('.evolve/phases/dependency-audit/phase.json') as f:
    d = json.load(f)
classify = d.get('classify')
assert classify is not None, 'classify must be defined'
has_sections = bool(classify.get('require_sections'))
has_signal = bool(classify.get('fail_if_signal'))
assert has_sections or has_signal, f'classify must have require_sections or fail_if_signal: {classify}'
print('PASS')
"
```
Expected: `PASS`

### AC6: Negative — dependency-audit does NOT appear before security-scan spec test [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  go test ./internal/phasespec/... -v 2>&1 | grep -E "^---" | grep FAIL | head -5
```
Expected: empty (no failing tests)

### AC7: Edge — phase validates (ValidateUserSpec returns no violations) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  ./evolve phases validate dependency-audit 2>&1
```
Expected: exit 0 or output containing `valid` (not `invalid` or `error`)
