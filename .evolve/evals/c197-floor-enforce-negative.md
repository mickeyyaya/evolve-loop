# Eval: c197-floor-enforce-negative

## Goal
Add two additive negative tests:
1. A new Enforce-mode scenario in `floor_activation_scenarios_test.go` proving
   that an advisor plan proposing scout→ship (skipping build+audit) at Enforce stage
   gets the floor-required phases inserted (the routing clamp is the "rejection").
2. A truncated-JSON negative test in `digest_test.go` proving that a structurally
   valid JSON prefix that terminates mid-object is handled gracefully (fail-open:
   Present=false, no error propagated).

## Acceptance Criteria

### AC-1: New Enforce scenario exists in floor_activation_scenarios_test.go [code]
```
grep -n "Enforce()" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/floor_activation_scenarios_test.go
```
Must produce at least one match — a new Enforce() scenario was added.

### AC-2: New truncated-JSON test exists in digest_test.go [code]
```
grep -n "truncat\|TruncatedJSON\|TruncatedArtifact\|partial.*json\|cut.*off" /Users/danleemh/ai/claude/evolve-loop/go/internal/router/digest_test.go
```
Must produce at least one match.

### AC-3: Truncated JSON test asserts fail-open behavior [code]
```
grep -A 8 "TruncatedJSON\|TruncatedArtifact\|truncat" /Users/danleemh/ai/claude/evolve-loop/go/internal/router/digest_test.go | grep "Present"
```
Must show `Present: false` or `Present` assertion — proving the test verifies fail-open.

### AC-4: All affected tests pass [code]
```
cd go && go test ./internal/core/... ./internal/router/... 2>&1 | grep -E "^ok|FAIL" | tail -10
```
Must show only `ok` lines, no FAIL.

## Negative Cases

### NEG-1: Truncated JSON (valid prefix) must NOT produce Present=true [code]
The digest fail-open contract says: any unparseable content → Present=false, no error.
A truncated JSON file like `{"action":"RESHIP"` (no closing brace) must yield Present=false:
```
cd go && go test -run "TestDigest.*Truncat\|TestDigest.*Chaos" ./internal/router/... -v 2>&1 | tail -10
```
Must pass (the new test asserts this).

### NEG-2: Floor scenario: advisor ship-without-build forces build insertion [code]
```
cd go && go test -run "TestFloorActivation" ./internal/core/... -v 2>&1 | grep -E "PASS|FAIL" | tail -10
```
Must show all PASS.
