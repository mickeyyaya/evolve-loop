# Eval: floor-violation-negative-test

## Task
Add explicit negative tests to `go/test/trustkernel/trustkernel_test.go` asserting that the routing integrity floor `ship ⇒ build ∧ audit` REJECTS a deficient plan. The existing positive test verifies the floor FORCES missing phases in; the new negative test verifies the forced clamps are specifically named ("build" and "audit") and that the non-configurable floor cannot be bypassed even when `cfg.Mandatory` is set to exclude build and audit.

## Acceptance Criteria

### AC1: New negative floor test exists and passes [code]
```
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./test/trustkernel/... -run TestFloor -v 2>&1 | grep -E "^=== RUN|^--- (PASS|FAIL)"
```
Expected: At least one `TestFloor*` test runs and all show `--- PASS`

### AC2: Clamps specifically name "build" and "audit" [code]
The test must assert clamps contain the strings "build" and "audit" (not just non-empty):
```
grep -n '"build"\|"audit"' /Users/danleemh/ai/claude/evolve-loop/go/test/trustkernel/trustkernel_test.go | tail -10
```
Expected: Lines where the test checks clamps contain "build" and "audit" specifically

### AC3: Negative case — mandatory-set bypass attempt is rejected [code]
```
grep -c "Mandatory\|mandatory.*bypass\|cfg.*Mandatory\|bypass.*floor\|non-configurable\|cannot.*bypass" /Users/danleemh/ai/claude/evolve-loop/go/test/trustkernel/trustkernel_test.go
```
Expected: `> 0` (test describes the non-configurable floor bypass attempt)

### AC4: All trustkernel tests pass [code]
```
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./test/trustkernel/... 2>&1 | tail -3
```
Expected: `ok  github.com/mickeyyaya/evolve-loop/go/test/trustkernel`

### AC5: Negative floor test does NOT call t.Skip [code]
The floor-violation test must be a hard assertion, not a skippable check:
```
grep -A20 "TestFloorViolation\|TestRoutingFloor_Negative\|TestFloor_Ship" /Users/danleemh/ai/claude/evolve-loop/go/test/trustkernel/trustkernel_test.go | grep -c "t.Skip"
```
Expected: `0` (the negative test never skips — it's a deterministic assertion)
