# Eval: orchestrator-floor-violation-negative-test

## Purpose
Verify a direct orchestrator-level negative test: a routing advisor that
returns a floor-violating plan (ship=true, build=false, audit=false) is
clamped before execution so build and audit actually run before ship.

## Code Graders [code]

- `grep -rn "floor.*violat\|FloorViolat\|violat.*floor\|ship.*without.*build\|FloorViolatingAdvisor\|floorViolat" go/internal/core/ --include="*_test.go" | grep -qv "^Binary"` — floor-violation negative test exists in core
- `cd go && go test ./internal/core/... 2>&1 | grep -c "^--- FAIL" | grep "^0$"` — all core tests pass
- `cd go && go test ./internal/core/... -run ".*[Ff]loor.*[Vv]iolat.*\|.*[Vv]iolat.*[Ff]loor.*" -v 2>&1 | grep -q "PASS"` — the floor-violation test runs and passes

## Regression Graders

- `cd go && go test ./internal/router/... 2>&1 | grep -c "^--- FAIL" | grep "^0$"` — router tests unaffected
- `cd go && go test ./test/trustkernel/... 2>&1 | grep -c "^--- FAIL" | grep "^0$"` — trustkernel tests unaffected

## Acceptance Notes
- Test must be at the ORCHESTRATOR level (not just the pure router.ClampPlanToFloor function)
- Use a fake advisor that returns {ship=true, build=false, audit=false} (floor-violating)
- Assert that build+audit both appear in PhasesRun before ship
- Distinct from existing router/floor_test.go (pure function) and trustkernel tests
