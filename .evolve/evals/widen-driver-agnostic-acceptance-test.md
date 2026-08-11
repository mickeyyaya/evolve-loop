# Eval: widen-driver-agnostic-acceptance-test

## Slug
`widen-driver-agnostic-acceptance-test`

## Goal
`TestSpineProfilesAreDriverAgnostic` (or a new `TestAllProfilesAreDriverAgnostic`) must cover ALL profiles in `.evolve/profiles/`, not just the 7 spine phases. The `docs/architecture/model-routing-policy.md` migration status section must reflect completed migration.

---

## Acceptance Criteria

### AC1 — All-profiles test exists and passes [code]
```bash
cd $PROJECT_ROOT/go
go test -count=1 ./internal/profiles/... 2>&1
```
Expected: `ok  github.com/mickeyyaya/evolve-loop/go/internal/profiles` with no FAIL lines

### AC2 — Test covers more than just the 7 spine profiles [code]
```bash
cd $PROJECT_ROOT
# Test either dynamically enumerates all profiles OR explicitly lists more than 7
file_content=$(cat go/internal/profiles/driver_agnostic_test.go)
# Check for dynamic enumeration (glob/ReadDir) or that at least 20 profile names appear
if echo "$file_content" | grep -qE 'ReadDir|Glob|glob|all.*profile|AllProfiles|TestAllProfiles'; then
    echo "PASS: test uses dynamic enumeration"
elif echo "$file_content" | grep -cE '"[a-z-]+"' | xargs -I{} test {} -ge 20; then
    echo "PASS: test lists 20+ profiles"
else
    echo "FAIL: test still only covers a narrow set"
    exit 1
fi
```
Expected output: starts with `PASS`

### AC3 — model-routing-policy.md reflects migration complete [code]
```bash
cd $PROJECT_ROOT
if grep -q "migration.*complet\|migrat.*done\|migrat.*finish\|all.*profiles.*canonical\|canonical.*all.*profiles" docs/architecture/model-routing-policy.md; then
    echo "PASS"
else
    echo "FAIL: migration-complete language not found in model-routing-policy.md"
    exit 1
fi
```
Expected output: `PASS`

### AC4 — Existing spine profiles still pass the guard [code]
```bash
cd $PROJECT_ROOT/go
go test -count=1 ./internal/profiles/... -run TestSpineProfilesAreDriverAgnostic -v 2>&1 | tail -5
```
Expected: PASS line with no FAIL

### AC5 — New test fails fast on a vendor-name injection [code]
```bash
cd $PROJECT_ROOT
# Temporarily inject a vendor name into a non-spine profile, confirm test catches it
cp .evolve/profiles/debugger.json .evolve/profiles/debugger.json.bak
python3 -c "
import json
d = json.load(open('.evolve/profiles/debugger.json'))
d['model_tier_default'] = 'sonnet'
json.dump(d, open('.evolve/profiles/debugger.json', 'w'), indent=2)
"
cd go
if go test -count=1 ./internal/profiles/... -run TestAllProfilesAreDriverAgnostic 2>&1 | grep -q "FAIL\|Error\|not a canonical"; then
    echo "PASS: test caught vendor injection"
else
    echo "FAIL: test did not catch vendor name injection"
fi
# Restore
cp $PROJECT_ROOT/.evolve/profiles/debugger.json.bak $PROJECT_ROOT/.evolve/profiles/debugger.json
```
Expected output: `PASS: test caught vendor injection`

### AC6 — test package compiles with no errors [code]
```bash
cd $PROJECT_ROOT/go
go build ./internal/profiles/... 2>&1 && echo "PASS" || echo "FAIL"
```
Expected output: `PASS`

---

## Gaming protection

The cheapest fake would be to pass a hardcoded empty list. AC5 (adversarial injection) verifies the test actually fails when a real profile has a vendor name. AC2 verifies breadth of coverage.
