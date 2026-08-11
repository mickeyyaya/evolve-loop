# Eval: all-profiles-substitutability-parity-test

## Slug
`all-profiles-substitutability-parity-test`

## Goal
Add `TestAllProfilesSubstitutabilityAtParity` to `go/internal/profiles/profile_model_routing_amplification_test.go`.
The function must use **real bridge manifests** (via `bridge.LoadManifest`) — not a synthetic fixture —
and must assert that for EVERY profile, EVERY canonical tier value (default + overrides + envelope)
resolves to a non-empty model in EACH swappable-driver manifest (codex, codex-tmux, agy, agy-tmux, ollama-tmux).
This closes the MEDIUM adversarial finding from cycle-340:
`TestSpineSubstitutabilityAtParity` verified 7 spine phases with a synthetic fixture; this test
covers all profiles with the same real bridge manifests that dispatch uses.

---

## Acceptance Criteria

### AC1 — New test exists and passes [code]
```bash
cd $PROJECT_ROOT/go
go test -count=1 -v ./internal/profiles/... -run TestAllProfilesSubstitutabilityAtParity 2>&1
```
Expected: line containing `--- PASS: TestAllProfilesSubstitutabilityAtParity`

### AC2 — Test covers more than 7 profiles [code]
```bash
cd $PROJECT_ROOT
# The test must iterate over all profiles via loader.List(), not a hardcoded list
grep -c '"scanner"\|"scout"\|"builder"\|"auditor"\|"triage"\|"tdd-engineer"\|"router"' \
    go/internal/profiles/profile_model_routing_amplification_test.go \
    | xargs -I{} test {} -le 10 && echo "PASS: no large hardcoded list" || echo "FAIL: appears to use a hardcoded list"
```
Expected output: `PASS: no large hardcoded list`

### AC3 — Test uses real bridge.LoadManifest (not synthetic fixture) [code]
```bash
cd $PROJECT_ROOT
# Must call bridge.LoadManifest in the new test function
grep -A 60 "func TestAllProfilesSubstitutabilityAtParity" go/internal/profiles/profile_model_routing_amplification_test.go \
    | grep -q "LoadManifest\|loadSwappableManifests\|swappableDriverManifests" && echo "PASS" || echo "FAIL: no real manifest loading found"
```
Expected output: `PASS`

### AC4 — Test uses t.Errorf not t.Skip for failures [code]
```bash
cd $PROJECT_ROOT
# Verify no t.Skip in the new function; verify t.Errorf is used
func_body=$(awk '/func TestAllProfilesSubstitutabilityAtParity/,/^}/' \
    go/internal/profiles/profile_model_routing_amplification_test.go)
if echo "$func_body" | grep -q "t.Skip"; then
    echo "FAIL: t.Skip found — misses must be failures"
    exit 1
fi
if echo "$func_body" | grep -q "t.Errorf\|t.Fatalf"; then
    echo "PASS"
else
    echo "FAIL: no t.Errorf/t.Fatalf found in function body"
    exit 1
fi
```
Expected output: `PASS`

### AC5 — Full profiles test suite still passes (no regression) [code]
```bash
cd $PROJECT_ROOT/go
go test -count=1 ./internal/profiles/... 2>&1
```
Expected: line `ok  github.com/mickeyyaya/evolve-loop/go/internal/profiles` with no FAIL

### AC6 — Test fails when a manifest tier entry is removed (adversarial injection) [code]
```bash
cd $PROJECT_ROOT
# Inject a bad tier key into the codex manifest (via env override), confirm test catches it
# We simulate by checking that the test enumerates all 3 canonical tiers per driver
func_body=$(awk '/func TestAllProfilesSubstitutabilityAtParity/,/^}/' \
    go/internal/profiles/profile_model_routing_amplification_test.go)
# The test must reference tier values from the profile (not hardcode "balanced")
if echo "$func_body" | grep -qE 'ModelTierDefault|model_tier|\.Tier|profile\.'; then
    echo "PASS: test reads tier from profile (not hardcoded)"
else
    echo "FAIL: test may use hardcoded tiers rather than reading from profiles"
    exit 1
fi
```
Expected output: `PASS: test reads tier from profile (not hardcoded)`

---

## Gaming protection

The cheapest fake would be an empty or skipped test. AC1 runs the real test in subprocess (no exit 0 shim).
AC2 verifies no large hardcoded list (dynamic enumeration only). AC3 verifies real manifest loading.
AC4 verifies failures are real errors, not skipped. AC6 verifies the tier is read from the profile, not hardcoded.
