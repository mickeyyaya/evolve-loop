# Eval: add-spec-verify-phase

## Objective
Verify that the spec-verify phase is fully scaffolded: agent prompt, profile, and phase-registry entry all present and coherent.

## Criteria

### C1 — Agent prompt file exists with required sections [code]
```bash
#!/bin/bash
set -euo pipefail
FILE="agents/evolve-spec-verifier.md"
test -f "$FILE" || { echo "FAIL: $FILE not found"; exit 1; }
SECTIONS=$(grep -c "^## " "$FILE" || true)
[ "$SECTIONS" -ge 3 ] || { echo "FAIL: agent prompt has only $SECTIONS sections, expected >= 3"; exit 1; }
# Must mention verification/validation of spec or acceptance criteria
grep -qi "acceptance.criteria\|spec.*verif\|predicate.*coverage\|verif.*spec" "$FILE" || {
  echo "FAIL: agent prompt does not mention spec verification"; exit 1;
}
echo "PASS: $FILE exists with $SECTIONS sections, mentions spec verification"
```

### C2 — Profile JSON exists and is valid [code]
```bash
#!/bin/bash
set -euo pipefail
FILE=".evolve/profiles/spec-verifier.json"
test -f "$FILE" || { echo "FAIL: $FILE not found"; exit 1; }
python3 -c "import json, sys; d=json.load(open('$FILE')); assert d.get('name') == 'spec-verifier', f'name={d.get(\"name\")}'; assert 'output_artifact' in d, 'missing output_artifact'; print('PASS: profile valid, name=spec-verifier, output_artifact=', d['output_artifact'])"
```

### C3 — Phase registry contains spec-verify entry [code]
```bash
#!/bin/bash
set -euo pipefail
python3 - <<'EOF'
import json, sys
d = json.load(open('docs/architecture/phase-registry.json'))
names = [p.get('name') for p in d.get('phases', [])]
if 'spec-verify' not in names:
    print(f"FAIL: 'spec-verify' not in phase registry. Found: {names}")
    sys.exit(1)
phase = next(p for p in d['phases'] if p.get('name') == 'spec-verify')
assert phase.get('optional') == True, "spec-verify must be optional=true"
print(f"PASS: spec-verify in registry, optional={phase.get('optional')}")
EOF
```

### C4 — Negative case: phase is optional (not mandatory default) [code]
```bash
#!/bin/bash
set -euo pipefail
python3 - <<'EOF'
import json, sys
d = json.load(open('docs/architecture/phase-registry.json'))
phase = next((p for p in d['phases'] if p.get('name') == 'spec-verify'), None)
if phase is None:
    print("FAIL: spec-verify not found in registry")
    sys.exit(1)
# Must NOT be in mandatory_phases config
mandatory = d.get('config', {}).get('mandatory_phases', [])
if 'spec-verify' in mandatory:
    print("FAIL: spec-verify should not be in mandatory_phases (it is optional)")
    sys.exit(1)
print("PASS: spec-verify correctly absent from mandatory_phases")
EOF
```

### C5 — Edge case: profile has bounded max_turns [code]
```bash
#!/bin/bash
set -euo pipefail
python3 - <<'EOF'
import json, sys
d = json.load(open('.evolve/profiles/spec-verifier.json'))
turns = d.get('max_turns', 0)
if turns <= 0 or turns > 20:
    print(f"FAIL: max_turns={turns} is out of reasonable range [1,20]")
    sys.exit(1)
print(f"PASS: max_turns={turns} is within bounds")
EOF
```
