# Eval: router-recipe-and-config

Verify that the goal-type recipe table was added to agents/evolve-router.md and that max_optional_insertions was bumped from 4 to 6 in docs/architecture/phase-registry.json.

## Criteria

### 1. max_optional_insertions is 6 in phase-registry.json [code]

```bash
VAL=$(python3 -c "import json; d=json.load(open('docs/architecture/phase-registry.json')); print(d['config']['max_optional_insertions'])")
[ "$VAL" = "6" ] && echo "PASS: max_optional_insertions=$VAL" || { echo "FAIL: max_optional_insertions=$VAL, want 6"; exit 1; }
```

### 2. Negative: max_optional_insertions is not 4 (old value) [code]

```bash
VAL=$(python3 -c "import json; d=json.load(open('docs/architecture/phase-registry.json')); print(d['config']['max_optional_insertions'])")
[ "$VAL" != "4" ] && echo "PASS: old value 4 not present" || { echo "FAIL: still has old value 4"; exit 1; }
```

### 3. phase-registry.json is still valid JSON after edit [code]

```bash
python3 -m json.tool docs/architecture/phase-registry.json > /dev/null 2>&1 && echo "PASS: valid JSON" || { echo "FAIL: invalid JSON"; exit 1; }
```

### 4. evolve-router.md contains goal-type recipe table with bugfix row [code]

```bash
grep -q "bugfix" agents/evolve-router.md && echo "PASS: bugfix row present" || { echo "FAIL: no bugfix row in evolve-router.md"; exit 1; }
```

### 5. evolve-router.md contains refactor row with smell-scan [code]

```bash
grep -q "smell-scan" agents/evolve-router.md && echo "PASS: smell-scan present in refactor recipe" || { echo "FAIL: no smell-scan in evolve-router.md"; exit 1; }
```

### 6. evolve-router.md contains fault-localization in bugfix recipe [code]

```bash
grep -q "fault-localization" agents/evolve-router.md && echo "PASS: fault-localization present" || { echo "FAIL: no fault-localization in evolve-router.md"; exit 1; }
```

### 7. evolve-router.md contains threat-model in security recipe [code]

```bash
grep -q "threat-model" agents/evolve-router.md && echo "PASS: threat-model present" || { echo "FAIL: no threat-model in evolve-router.md"; exit 1; }
```

### 8. Edge: recipe table covers at minimum 5 goal types [code]

```bash
COUNT=0
for goal in bugfix feature refactor security performance release; do
  grep -q "$goal" agents/evolve-router.md && COUNT=$((COUNT+1))
done
[ "$COUNT" -ge 5 ] && echo "PASS: $COUNT/6 goal types present" || { echo "FAIL: only $COUNT goal types covered (need >=5)"; exit 1; }
```
