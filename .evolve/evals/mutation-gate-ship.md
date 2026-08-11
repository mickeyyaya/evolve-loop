# Eval: mutation-gate-ship

## Task Summary
Ship the mutation-gate user phase to main: commit untracked phase.json, agent.md, and
profiles/mutation-gate.json; write cycle-224 ACS predicates including a behavioral H2
replacement for the grep-only persona check; drop micro-phase-wave-2 from carryoverTodos.

## Acceptance Criteria

### AC1 — Phase files committed and tracked [code]
```bash
cd "$(git rev-parse --show-toplevel)"
git ls-files --error-unmatch .evolve/phases/mutation-gate/phase.json
git ls-files --error-unmatch .evolve/phases/mutation-gate/agent.md
git ls-files --error-unmatch .evolve/profiles/mutation-gate.json
echo "GREEN: mutation-gate files are committed and git-tracked"
```

### AC2 — Phase validates via runtime [code]
```bash
cd "$(git rev-parse --show-toplevel)"
OUT=$(./go/bin/evolve phases validate mutation-gate 2>&1)
echo "$OUT" | grep -qi "^OK" || { echo "RED: phases validate did not return OK — $OUT" >&2; exit 1; }
echo "GREEN: mutation-gate validates: $OUT"
```

### AC3 — Phase appears in phases list [code]
```bash
cd "$(git rev-parse --show-toplevel)"
OUT=$(./go/bin/evolve phases list 2>&1)
echo "$OUT" | grep -q "mutation-gate" || { echo "RED: mutation-gate absent from phases list" >&2; exit 1; }
echo "GREEN: mutation-gate is in phases list"
```

### AC4 — Behavioral ACS predicate for persona (not grep-only) [code]
```bash
cd "$(git rev-parse --show-toplevel)"
P=$(ls acs/cycle-224/*mutation-gate*.sh 2>/dev/null | head -1)
[ -n "$P" ] || { echo "RED: no mutation-gate ACS predicate found in acs/cycle-224/" >&2; exit 1; }
# Must NOT be marked as grep_only; must be behavioral (config-check or behavioral class)
grep -qi "acs-predicate: grep_only" "$P" && { echo "RED: predicate $P is still classified grep_only" >&2; exit 1; }
# The predicate must invoke a runtime command (not only grep)
grep -qE "evolve|go/bin" "$P" || { echo "RED: predicate $P has no runtime command call" >&2; exit 1; }
echo "GREEN: mutation-gate ACS predicate $P is behavioral (not grep_only)"
```

### AC5 — ACS suite for cycle-224 is green [code]
```bash
cd "$(git rev-parse --show-toplevel)"
OUT=$(./go/bin/evolve acs suite --cycle 224 2>&1)
echo "$OUT" | grep -q "verdict=PASS" || { echo "RED: ACS suite cycle-224 not PASS: $OUT" >&2; exit 1; }
echo "$OUT" | grep -qE "red=0" || { echo "RED: ACS suite has red predicates: $OUT" >&2; exit 1; }
echo "GREEN: ACS suite cycle-224: $OUT"
```

### AC6 — micro-phase-wave-2 removed from carryoverTodos [code]
```bash
cd "$(git rev-parse --show-toplevel)"
python3 -c "
import json, sys
with open('.evolve/state.json') as f:
    d = json.load(f)
ids = [t['id'] for t in d.get('carryoverTodos', [])]
if 'micro-phase-wave-2' in ids:
    print('RED: micro-phase-wave-2 still in carryoverTodos after ship', file=sys.stderr)
    sys.exit(1)
print('GREEN: micro-phase-wave-2 not in carryoverTodos')
"
```

### AC7 (negative) — Phase does NOT validate with a non-existent phase name [code]
```bash
cd "$(git rev-parse --show-toplevel)"
OUT=$(./go/bin/evolve phases validate nonexistent-phase-xyzzy 2>&1) && {
  echo "RED: validate accepted nonexistent phase (gaming vector)" >&2; exit 1
} || true
echo "GREEN: phases validate correctly rejects unknown phases"
```
