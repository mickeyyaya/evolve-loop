# Eval: migrate-domain-profiles-to-canonical-tiers

## Slug
`migrate-domain-profiles-to-canonical-tiers`

## Goal
All `.evolve/profiles/*.json` files must express `model_tier_default`, `model_tier_overrides` values, and `model_tier_envelope` fields exclusively with canonical tiers (`fast` | `balanced` | `deep`). No vendor model names (`sonnet`, `opus`, `haiku`) may appear.

---

## Acceptance Criteria

### AC1 — No vendor names in model_tier_default [code]
```bash
count=$(grep -r '"model_tier_default": "sonnet"\|"model_tier_default": "opus"\|"model_tier_default": "haiku"' /path/to/.evolve/profiles/ | wc -l | tr -d ' ')
[ "$count" = "0" ] && echo "PASS" || echo "FAIL: $count profiles still have vendor model names"
```
Expected output: `PASS`

### AC2 — No vendor names in model_tier_overrides [code]
```bash
cd $PROJECT_ROOT
python3 -c "
import json, glob, sys
vendor = {'sonnet','opus','haiku'}
failures = []
for f in sorted(glob.glob('.evolve/profiles/*.json')):
    d = json.load(open(f))
    for k, v in d.get('model_tier_overrides', {}).items():
        if v in vendor:
            failures.append(f'{f}: overrides[{k}]={v}')
if failures:
    print('FAIL:')
    for x in failures: print(' ', x)
    sys.exit(1)
else:
    print('PASS')
"
```
Expected output: `PASS`

### AC3 — Canonical tiers only in envelope bounds [code]
```bash
cd $PROJECT_ROOT
python3 -c "
import json, glob, sys
canonical = {'fast','balanced','deep',''}
vendor = {'sonnet','opus','haiku'}
failures = []
for f in sorted(glob.glob('.evolve/profiles/*.json')):
    d = json.load(open(f))
    env = d.get('model_tier_envelope', {})
    for field in ('min','default','max'):
        v = env.get(field,'')
        if v in vendor:
            failures.append(f'{f}: envelope.{field}={v}')
if failures:
    print('FAIL:')
    for x in failures: print(' ', x)
    sys.exit(1)
else:
    print('PASS')
"
```
Expected output: `PASS`

### AC4 — memo and evaluator use fast tier (edge cases) [code]
```bash
cd $PROJECT_ROOT
python3 -c "
import json
for name in ('memo','evaluator'):
    d = json.load(open(f'.evolve/profiles/{name}.json'))
    tier = d.get('model_tier_default','')
    if tier != 'fast':
        print(f'FAIL: {name} model_tier_default={tier!r}, want fast')
        exit(1)
print('PASS')
"
```
Expected output: `PASS`

### AC5 — plan-reviewer and retrospective use deep tier (envelope-driven overrides) [code]
```bash
cd $PROJECT_ROOT
python3 -c "
import json
for name in ('plan-reviewer','retrospective'):
    d = json.load(open(f'.evolve/profiles/{name}.json'))
    tier = d.get('model_tier_default','')
    if tier != 'deep':
        print(f'FAIL: {name} model_tier_default={tier!r}, want deep')
        exit(1)
print('PASS')
"
```
Expected output: `PASS`

### AC6 — All profiles parse as valid JSON [code]
```bash
cd $PROJECT_ROOT
python3 -c "
import json, glob, sys
errors = []
for f in sorted(glob.glob('.evolve/profiles/*.json')):
    try:
        json.load(open(f))
    except json.JSONDecodeError as e:
        errors.append(f'{f}: {e}')
if errors:
    print('FAIL:', errors)
    sys.exit(1)
print('PASS')
"
```
Expected output: `PASS`

### AC7 — Negative: vendor name "sonnet" does NOT appear in any profile tier field [code]
```bash
cd $PROJECT_ROOT
if grep -r '"model_tier_default": "sonnet"\|"model_tier_default": "opus"\|"model_tier_default": "haiku"' .evolve/profiles/ 2>/dev/null | grep -q .; then
  echo "FAIL: vendor model names still present"
  exit 1
else
  echo "PASS"
fi
```
Expected output: `PASS`

---

## Gaming protection

The cheapest fake would be to delete profiles rather than migrate them. AC6 verifying all profiles parse as valid JSON, combined with the existing `TestSpineProfilesAreDriverAgnostic` guard, prevents deletions from passing silently.
