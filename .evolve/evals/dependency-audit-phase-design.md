# Eval: dependency-audit-phase-design

## Task
Builder registers a `dependency-audit` phase in `docs/architecture/phase-registry.json` and writes its agent spec at `agents/evolve-dependency-audit.md`. The phase must be advisor-routable (optional), cross-CLI portable, and not introduce a new always-on env toggle.

## Acceptance Criteria

### C1 — phase registered in phase-registry.json [code]
```bash
python3 -c "
import json, sys
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
names = [p['name'] for p in d.get('phases', [])]
assert 'dependency-audit' in names, 'dependency-audit not in phase registry'
print('PASS')
"
```
Expected: `PASS`

### C2 — phase is marked optional (advisor-routable) [code]
```bash
python3 -c "
import json
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
phase = next((p for p in d['phases'] if p['name'] == 'dependency-audit'), None)
assert phase is not None, 'phase not found'
assert phase.get('optional', False) == True, 'phase must be optional'
print('PASS')
"
```
Expected: `PASS`

### C3 — agent spec file exists [code]
```bash
test -f agents/evolve-dependency-audit.md && echo PASS || echo FAIL
```
Expected: `PASS`

### C4 — agent spec references at least one real dependency-scan command [code]
```bash
grep -iE "govulncheck|npm audit|pip.audit|go mod|trivy|osv.dev|grype|cyclonedx" agents/evolve-dependency-audit.md | wc -l | tr -d ' '
```
Expected: integer ≥ 1

### C5 — phase-registry.json still parses as valid JSON after edit [code]
```bash
python3 -c "import json; json.load(open('docs/architecture/phase-registry.json')); print('VALID')"
```
Expected: `VALID`

### C6 — no new env toggle added for this phase (no flag sprawl) [code]
```bash
grep -E "EVOLVE_DEPENDENCY_AUDIT_" docs/architecture/phase-registry.json agents/evolve-dependency-audit.md 2>/dev/null | wc -l | tr -d ' '
```
Expected: `0`

### C7 (negative) — phase is NOT mandatory [code]
```bash
python3 -c "
import json
with open('docs/architecture/phase-registry.json') as f:
    d = json.load(f)
phase = next((p for p in d['phases'] if p['name'] == 'dependency-audit'), None)
mandatory = phase.get('mandatory', False) if phase else False
assert not mandatory, 'phase must not be mandatory'
print('PASS')
"
```
Expected: `PASS`
