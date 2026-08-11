# Eval: mutation-gate-phase-impl
<!-- cycle: 223 -->

## Summary
Verify that the `mutation-gate` user phase is correctly authored, validates, and registers in the phase registry.

## Acceptance Criteria

### AC-1: Phase files exist [code]
```bash
test -f .evolve/phases/mutation-gate/phase.json && \
test -f .evolve/phases/mutation-gate/agent.md && \
echo "PASS: phase files exist"
```
Expected: `PASS: phase files exist`

### AC-2: Phase validates successfully [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
evolve phases validate mutation-gate 2>&1 | tail -3
```
Expected output contains: `OK` or `valid` (not `ERROR` or `FAIL`)

### AC-3: Phase appears in phases list [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
evolve phases list 2>&1 | grep mutation-gate
```
Expected: line containing `mutation-gate`

### AC-4: Permission profile exists [code]
```bash
test -f .evolve/profiles/mutation-gate.json && \
python3 -c "import json,sys; d=json.load(open('.evolve/profiles/mutation-gate.json')); sys.exit(0 if d.get('name')=='mutation-gate' else 1)" && \
echo "PASS: profile valid"
```
Expected: `PASS: profile valid`

### AC-5: phase.json contains required signal outputs [code]
```bash
python3 -c "
import json, sys
d = json.load(open('.evolve/phases/mutation-gate/phase.json'))
sigs = d.get('outputs', {}).get('signals', [])
required = ['mutation.score', 'mutation.survivors']
missing = [s for s in required if s not in sigs]
if missing:
    print('MISSING:', missing); sys.exit(1)
print('PASS: signals present')
"
```
Expected: `PASS: signals present`

### AC-6: phase.json has fail_if_signal for mutation.score < 60 [code]
```bash
python3 -c "
import json, sys
d = json.load(open('.evolve/phases/mutation-gate/phase.json'))
fis = d.get('classify', {}).get('fail_if_signal', {})
val = fis.get('mutation.score', '')
if val != '<60':
    print('FAIL: expected fail_if_signal mutation.score=<60, got:', val); sys.exit(1)
print('PASS: gate threshold correct')
"
```
Expected: `PASS: gate threshold correct`

### AC-7 (negative): phase.json must not have writes_source=true [code]
```bash
python3 -c "
import json, sys
d = json.load(open('.evolve/phases/mutation-gate/phase.json'))
if d.get('writes_source', False):
    print('FAIL: writes_source must be false for evaluate archetype'); sys.exit(1)
print('PASS: writes_source correctly false')
"
```
Expected: `PASS: writes_source correctly false`

### AC-8 (edge): agent.md must contain required sections [code]
```bash
python3 -c "
import sys
content = open('.evolve/phases/mutation-gate/agent.md').read()
required = ['## Summary', '## Survivors', '## Verdict']
missing = [s for s in required if s not in content]
if missing:
    # Check output-format line instead (frontmatter specifies section names)
    pass
# Check that agent.md has frontmatter with output-format
if 'output-format' not in content:
    print('FAIL: agent.md missing output-format frontmatter key'); sys.exit(1)
print('PASS: agent.md structure valid')
"
```
Expected: `PASS: agent.md structure valid`
