# Eval: profile-provenance-field

## Task
Add `generated_from` provenance field to all `.evolve/profiles/*.json` files and validate it is present.

## Criteria

### C1: Go struct field exists [code]
```bash
grep -q '"generated_from"' /Users/danleemh/ai/claude/evolve-loop/go/internal/profiles/profiles.go
```
Expected: exit 0

### C2: All profiles carry generated_from field [code]
```bash
missing=$(for f in /Users/danleemh/ai/claude/evolve-loop/.evolve/profiles/*.json; do
  python3 -c "import json,sys; d=json.load(open('$f')); sys.exit(0 if 'generated_from' in d else 1)" 2>/dev/null || echo "$f"
done)
[ -z "$missing" ] && echo "PASS: all profiles have generated_from" || { echo "FAIL: missing in: $missing"; exit 1; }
```
Expected: exit 0, "PASS: all profiles have generated_from"

### C3: evolve phases validate reports no missing provenance [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && go/evolve phases validate 2>&1 | grep -c "missing.*generated_from" | grep -q "^0$" && echo "PASS: no provenance warnings" || { echo "FAIL: provenance warnings found"; exit 1; }
```
Expected: exit 0, no "missing generated_from" lines

### C4: Negative — profile without generated_from fails strict-provenance check [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  TMP=$(mktemp -d) && \
  python3 -c "import json; d=json.load(open('.evolve/profiles/scout.json')); d.pop('generated_from', None); json.dump(d, open('$TMP/scout.json','w'))" && \
  EVOLVE_PROFILE_DIR="$TMP" go/evolve phases validate --strict-provenance 2>&1 | grep -q "generated_from" && echo "PASS: detected missing field" || { echo "FAIL: did not detect missing field"; exit 1; }
```
Expected: exit 0, "PASS: detected missing field"

### C5: Go tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/profiles/... -count=1 -run TestProvenance 2>&1
```
Expected: exit 0, PASS

### C6: ACS predicate exists and passes [code]
```bash
test -f /Users/danleemh/ai/claude/evolve-loop/acs/cycle-239/000-profile-provenance.sh && \
  bash /Users/danleemh/ai/claude/evolve-loop/acs/cycle-239/000-profile-provenance.sh 2>&1
```
Expected: exit 0

### C7 (regression): Full test suite green [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... -count=1 2>&1 | grep "^FAIL" | head -5
```
Expected: no output (no failures)
