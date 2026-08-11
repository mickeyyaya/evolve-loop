# Eval: wave1-phase-files

Verify that all 7 Wave 1 micro-phase directories exist with valid phase.json, agent.md, and profile.json files.

## Criteria

### 1. All phase directories exist [code]

```bash
PHASES="fault-localization reproduce-bug behavior-baseline behavior-compare smell-scan threat-model test-amplification"
MISSING=0
for p in $PHASES; do
  if [ ! -d ".evolve/phases/$p" ]; then
    echo "MISSING: .evolve/phases/$p"
    MISSING=$((MISSING+1))
  fi
done
[ "$MISSING" -eq 0 ] && echo "PASS: all 7 phase dirs exist" || { echo "FAIL: $MISSING dirs missing"; exit 1; }
```

### 2. Each phase.json has required fields [code]

```bash
PHASES="fault-localization reproduce-bug behavior-baseline behavior-compare smell-scan threat-model test-amplification"
FAIL=0
for p in $PHASES; do
  f=".evolve/phases/$p/phase.json"
  if [ ! -f "$f" ]; then
    echo "MISSING: $f"
    FAIL=$((FAIL+1))
    continue
  fi
  # must have optional: true
  if ! python3 -c "import json; d=json.load(open('$f')); assert d.get('optional') == True, 'optional!=true'" 2>/dev/null; then
    echo "FAIL optional!=true: $f"
    FAIL=$((FAIL+1))
  fi
  # must have classify
  if ! python3 -c "import json; d=json.load(open('$f')); assert 'classify' in d, 'no classify'" 2>/dev/null; then
    echo "FAIL no classify: $f"
    FAIL=$((FAIL+1))
  fi
  # must have routing.insert_when
  if ! python3 -c "import json; d=json.load(open('$f')); assert 'routing' in d and 'insert_when' in d['routing'], 'no routing'" 2>/dev/null; then
    echo "FAIL no routing.insert_when: $f"
    FAIL=$((FAIL+1))
  fi
done
[ "$FAIL" -eq 0 ] && echo "PASS: all phase.json valid" || { echo "FAIL: $FAIL violations"; exit 1; }
```

### 3. Each agent.md exists [code]

```bash
PHASES="fault-localization reproduce-bug behavior-baseline behavior-compare smell-scan threat-model test-amplification"
MISSING=0
for p in $PHASES; do
  if [ ! -f ".evolve/phases/$p/agent.md" ]; then
    echo "MISSING: .evolve/phases/$p/agent.md"
    MISSING=$((MISSING+1))
  fi
done
[ "$MISSING" -eq 0 ] && echo "PASS: all agent.md exist" || { echo "FAIL: $MISSING missing"; exit 1; }
```

### 4. Each agent.md has frontmatter name field [code]

```bash
PHASES="fault-localization reproduce-bug behavior-baseline behavior-compare smell-scan threat-model test-amplification"
FAIL=0
for p in $PHASES; do
  f=".evolve/phases/$p/agent.md"
  [ ! -f "$f" ] && continue
  if ! grep -q "^name: evolve-$p" "$f"; then
    echo "FAIL missing frontmatter name evolve-$p in $f"
    FAIL=$((FAIL+1))
  fi
done
[ "$FAIL" -eq 0 ] && echo "PASS: all agent.md have correct frontmatter" || { echo "FAIL: $FAIL bad frontmatter"; exit 1; }
```

### 5. Negative: no phase has optional: false [code]

```bash
# A user phase with optional:false would violate the floor — must not exist
FOUND=$(grep -r '"optional": false' .evolve/phases/fault-localization .evolve/phases/reproduce-bug .evolve/phases/behavior-baseline .evolve/phases/behavior-compare .evolve/phases/smell-scan .evolve/phases/threat-model .evolve/phases/test-amplification 2>/dev/null | grep phase.json | wc -l)
[ "$FOUND" -eq 0 ] && echo "PASS: no floor violation (no optional:false)" || { echo "FAIL: $FOUND phases have optional:false"; exit 1; }
```

### 6. Negative: phase.json is valid JSON (not malformed) [code]

```bash
PHASES="fault-localization reproduce-bug behavior-baseline behavior-compare smell-scan threat-model test-amplification"
FAIL=0
for p in $PHASES; do
  f=".evolve/phases/$p/phase.json"
  [ ! -f "$f" ] && continue
  if ! python3 -m json.tool "$f" > /dev/null 2>&1; then
    echo "FAIL invalid JSON: $f"
    FAIL=$((FAIL+1))
  fi
done
[ "$FAIL" -eq 0 ] && echo "PASS: all phase.json are valid JSON" || { echo "FAIL: $FAIL invalid"; exit 1; }
```

### 7. Edge: classify.require_sections is non-empty for each phase [code]

```bash
PHASES="fault-localization reproduce-bug behavior-baseline behavior-compare smell-scan threat-model test-amplification"
FAIL=0
for p in $PHASES; do
  f=".evolve/phases/$p/phase.json"
  [ ! -f "$f" ] && continue
  COUNT=$(python3 -c "import json; d=json.load(open('$f')); print(len(d.get('classify',{}).get('require_sections',[])))" 2>/dev/null)
  if [ -z "$COUNT" ] || [ "$COUNT" -lt 1 ]; then
    echo "FAIL empty require_sections: $f"
    FAIL=$((FAIL+1))
  fi
done
[ "$FAIL" -eq 0 ] && echo "PASS: all phases have non-empty require_sections" || { echo "FAIL: $FAIL empty"; exit 1; }
```
