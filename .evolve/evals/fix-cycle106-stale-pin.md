# Eval: fix-cycle106-stale-pin

## Task
Fix the stale `TestC106_011_BinaryVersionIsV12_1_1` version pin in `go/acs/cycle106/predicates_test.go`. The test currently checks the binary version contains "12.1.1" but the binary is at v16.x ("devel"), causing an active FAIL.

## Acceptance Criteria

### AC1: cycle106 tests pass [code]
```
cd go && go test ./acs/cycle106/... ; echo "exit=$?"
```
Expected: `ok  github.com/mickeyyaya/evolve-loop/go/acs/cycle106` and `exit=0`

### AC2: version check no longer pins "12.1.1" literal [code]
```
grep -c '"12.1.1"' /Users/danleemh/ai/claude/evolve-loop/go/acs/cycle106/predicates_test.go
```
Expected: `0` (literal "12.1.1" string removed from the version assertion)

### AC3: binary version check still rejects RC suffix [code]
```
grep -c 'rc\|RC' /Users/danleemh/ai/claude/evolve-loop/go/acs/cycle106/predicates_test.go | head -3
grep -c 'rc\b\|RC\b\|rc4\|rc3' /Users/danleemh/ai/claude/evolve-loop/go/acs/cycle106/predicates_test.go
```
Expected: RC-suffix check still present in test (some non-zero grep, or the test still checks for absence of RC)

### AC4: negative case — RC suffix correctly rejected [code]
The test must not accept a version string with RC suffix. Verify the test logic still includes a rejection clause for versions containing "rc" or "RC":
```
grep -c 'rc\|RC' /Users/danleemh/ai/claude/evolve-loop/go/acs/cycle106/predicates_test.go
```
Expected: `> 0` (the no-RC-suffix invariant is still expressed in the test)
