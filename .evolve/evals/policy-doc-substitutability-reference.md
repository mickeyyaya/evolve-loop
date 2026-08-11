# Eval: policy-doc-substitutability-reference

## Slug
`policy-doc-substitutability-reference`

## Goal
Update `docs/architecture/model-routing-policy.md` so:
1. The "Substitutability acceptance test" paragraph cites `TestAllProfilesSubstitutabilityAtParity` (from cycle-341)
   as the definitive all-profiles parity guard, in addition to `TestSpineProfilesAreDriverAgnostic`.
2. The `Verification` command block includes the full suite command that covers the new test.
3. A brief note documents that 3 profiles (`builder`, `tdd-engineer`, `tester`) have intentional `allowed_clis`
   restrictions for cross-family / TDD-floor reasons — the MODEL TIER vocabulary is still driver-agnostic
   (tiers resolve for all drivers) even though DISPATCH gates restrict runtime driver selection.

---

## Acceptance Criteria

### AC1 — Policy doc cites TestAllProfilesSubstitutabilityAtParity [code]
```bash
cd $PROJECT_ROOT
grep -q "TestAllProfilesSubstitutabilityAtParity" docs/architecture/model-routing-policy.md \
    && echo "PASS" || echo "FAIL: new test not cited in policy doc"
```
Expected output: `PASS`

### AC2 — Allowed_clis exceptions documented [code]
```bash
cd $PROJECT_ROOT
# The policy doc must mention allowed_clis restrictions or exceptions
grep -qE "allowed_clis|dispatch.*restrict|cli.*restrict|tdd-engineer|tester.*claude" \
    docs/architecture/model-routing-policy.md \
    && echo "PASS" || echo "FAIL: allowed_clis dispatch exceptions not documented"
```
Expected output: `PASS`

### AC3 — Verification command still works [code]
```bash
cd $PROJECT_ROOT/go
go test -count=1 ./internal/profiles/... ./internal/resolvellm/... ./internal/modelcatalog/... 2>&1
```
Expected: all three `ok` lines, no FAIL

### AC4 — Old spine test still referenced (no regression) [code]
```bash
cd $PROJECT_ROOT
grep -q "TestSpineProfilesAreDriverAgnostic\|TestAllProfilesAreDriverAgnostic" \
    docs/architecture/model-routing-policy.md \
    && echo "PASS" || echo "FAIL: existing test reference removed from policy doc"
```
Expected output: `PASS`

### AC5 — Policy doc does not reintroduce vendor model names [code]
```bash
cd $PROJECT_ROOT
# Policy doc must not suggest vendor model names as tier values
if grep -qE 'model_tier_default.*haiku|model_tier_default.*sonnet|model_tier_default.*opus' \
    docs/architecture/model-routing-policy.md; then
    echo "FAIL: vendor model name found in policy doc"
    exit 1
else
    echo "PASS"
fi
```
Expected output: `PASS`

---

## Gaming protection

AC1 verifies the specific new test name is cited (not just any test). AC2 verifies the allowed_clis exception is acknowledged (can't just delete the section). AC4 ensures the existing test references aren't dropped. AC5 is a negative check — ensure the edit doesn't introduce model names.
