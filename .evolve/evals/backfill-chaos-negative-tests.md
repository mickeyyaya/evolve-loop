# Eval: backfill-chaos-negative-tests

## Goal
Add chaos negative tests to `go/internal/backfill/backfill_test.go` for
inputs the existing suite does not cover:
1. Binary/garbage content in clean.txt (no header, random bytes) — must
   return (false, nil) without panic.
2. Truncated clean.txt: header present but content cut off before minLen —
   must return (false, nil).
3. Header appears at end-of-file with zero body — must return (false, nil).

Also add a negative test to `go/test/trustkernel/trustkernel_test.go` that
verifies the routing floor specifically REJECTS (by clamping to include) a
plan that attempts ship without build and audit — the "phase-gate
floor-violation" negative test.

## Criteria

### C1 — backfill package tests all pass [code]
```bash
cd go && go test -v -count=1 ./internal/backfill/... 2>&1
```
Expected: exit 0, all tests PASS, including the new chaos tests.

### C2 — New chaos test names are present in the test file [code]
```bash
grep -c "func Test.*Chaos\|func Test.*Garbage\|func Test.*Binary\|func Test.*Truncat" \
  go/internal/backfill/backfill_test.go
```
Expected: count >= 1 (at least one chaos/truncated/binary test).

### C3 — trustkernel floor-violation negative test passes [code]
```bash
cd go && go test -v -count=1 -run TestRoutingFloor ./test/trustkernel/... 2>&1
```
Expected: all TestRoutingFloor_* tests PASS, including the existing
`TestRoutingFloor_ShipRequiresBuildAndAudit`.

### C4 — Binary/garbage content does not panic [code]
```bash
# The new test must have binary or null-byte content seeded as clean.txt.
grep -n "[]byte{0\|\\\\x00\|binary\|garbage\|rand\|Binary\|Garbage" \
  go/internal/backfill/backfill_test.go
```
Expected: at least one match (binary/garbage content explicitly tested).

### NEG-1 — Chaos test is not tautological (it can fail) [model]
A tautological test just calls `TryExtract` and checks `ok==false` with no
seeded clean.txt. If there is no input, `TryExtract` trivially returns false.
Verify the new chaos tests explicitly SEED a malformed/truncated file so the
test would fail if TryExtract panicked or returned ok=true. Manual review.

### NEG-2 — Floor-violation test is adversarial [code]
```bash
grep -A10 "ShipRequiresBuildAndAudit\|floor.*viol\|floor_viol" \
  go/test/trustkernel/trustkernel_test.go | head -20
```
Expected: the test plan deliberately omits build/audit (e.g., only has
`{Phase: "ship", Run: true}`) and verifies they get added by ClampPlanToFloor.
