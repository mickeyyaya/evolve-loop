# Eval: add-doc-sync-phase

## Objective
Verify that the doc-sync phase is fully scaffolded: agent prompt, profile, and phase-registry entry all present and coherent.

## Criteria

### C1 — Agent prompt file exists with documentation focus [code]
```bash
#!/bin/bash
set -euo pipefail
FILE="agents/evolve-doc-sync.md"
test -f "$FILE" || { echo "FAIL: $FILE not found"; exit 1; }
SECTIONS=$(grep -c "^## " "$FILE" || true)
[ "$SECTIONS" -ge 3 ] || { echo "FAIL: agent prompt has only $SECTIONS sections, expected >= 3"; exit 1; }
# Must mention documentation generation or changelog
grep -qi "documentation\|changelog\|doc.*generat\|api.*doc\|readme" "$FILE" || {
  echo "FAIL: agent prompt does not mention documentation generation"; exit 1;
}
echo "PASS: $FILE exists with $SECTIONS sections, mentions documentation"
```

### C2 — Profile JSON exists and is valid [code]
```bash
#!/bin/bash
set -euo pipefail
FILE=".evolve/profiles/doc-sync.json"
test -f "$FILE" || { echo "FAIL: $FILE not found"; exit 1; }
python3 -c "import json, sys; d=json.load(open('$FILE')); assert d.get('name') == 'doc-sync', f'name={d.get(\"name\")}'; assert 'output_artifact' in d, 'missing output_artifact'; print('PASS: profile valid, name=doc-sync, output_artifact=', d['output_artifact'])"
```

### C3 — Phase registry contains doc-sync entry [code]
```bash
#!/bin/bash
set -euo pipefail
python3 - <<'EOF'
import json, sys
d = json.load(open('docs/architecture/phase-registry.json'))
names = [p.get('name') for p in d.get('phases', [])]
if 'doc-sync' not in names:
    print(f"FAIL: 'doc-sync' not in phase registry. Found: {names}")
    sys.exit(1)
phase = next(p for p in d['phases'] if p.get('name') == 'doc-sync')
assert phase.get('optional') == True, "doc-sync must be optional=true"
print(f"PASS: doc-sync in registry, optional={phase.get('optional')}")
EOF
```

### C4 — Negative case: doc-sync placed AFTER build in pipeline ordering [code]
```bash
#!/bin/bash
set -euo pipefail
python3 - <<'EOF'
import json, sys
d = json.load(open('docs/architecture/phase-registry.json'))
phases = d.get('phases', [])
names = [p.get('name') for p in phases]
build_idx = next((i for i, p in enumerate(phases) if p.get('name') == 'build'), None)
doc_idx = next((i for i, p in enumerate(phases) if p.get('name') == 'doc-sync'), None)
if build_idx is None:
    print("FAIL: 'build' phase not found in registry")
    sys.exit(1)
if doc_idx is None:
    print("FAIL: 'doc-sync' not found in registry")
    sys.exit(1)
if doc_idx <= build_idx:
    print(f"FAIL: doc-sync (idx={doc_idx}) is before build (idx={build_idx}); must come after")
    sys.exit(1)
print(f"PASS: doc-sync (idx={doc_idx}) is after build (idx={build_idx})")
EOF
```

### C5 — Edge case: profile has write permissions to docs/ [code]
```bash
#!/bin/bash
set -euo pipefail
python3 - <<'EOF'
import json, sys
d = json.load(open('.evolve/profiles/doc-sync.json'))
# Either allowed_tools contains a Write for docs paths, or sandbox.write_subpaths includes docs or cycle workspace
allowed = d.get('allowed_tools', [])
sandbox_write = d.get('sandbox', {}).get('write_subpaths', [])
combined = allowed + sandbox_write
# Check that doc-sync can write either docs/ or cycle workspace
can_write_cycle = any('.evolve/runs/cycle-' in s or 'cycle-*' in s for s in combined)
can_write_docs = any('docs/' in s or 'knowledge-base' in s for s in combined)
if not (can_write_cycle or can_write_docs):
    print(f"FAIL: doc-sync profile does not grant any write access (combined paths: {combined})")
    sys.exit(1)
print(f"PASS: doc-sync has write access (cycle={can_write_cycle}, docs={can_write_docs})")
EOF
```
