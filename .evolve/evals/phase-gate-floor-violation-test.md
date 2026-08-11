# Eval: phase-gate-floor-violation-test

## Acceptance Criteria

### AC-1: New floor-violation negative test exists in core or router package [code]
```bash
grep -rq "TestFloorViolation\|TestShipWithoutBuild\|TestShipWithoutAudit\|TestFloorViolating\|TestOrchestrator.*FloorViolat\|TestSpineSatisfied.*FloorViolat\|TestShip_FloorViolat\|TestRoute.*FloorViolat" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/core/ \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/router/ 2>/dev/null
```

### AC-2: Test directly asserts ship is rejected when floor prerequisites absent [code]
```bash
# The test body must assert on rejection / error / block
grep -A 20 "TestFloorViolation\|TestShipWithoutBuild\|TestShipWithoutAudit\|TestFloorViolating\|TestShip_FloorViolat" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/core/*.go \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/router/*.go 2>/dev/null | \
  grep -i "false\|reject\|block\|Error\|FAIL\|notRun\|want.*false" | head -5
```

### AC-3: Test names are not tautological (not just checking SpineSatisfied=false) [code]
```bash
# Test must involve either the orchestrator routing the ship attempt
# OR the statemachine gate working end-to-end
# Verify: NOT just "sm.SpineSatisfiedUpTo returns false" — must test orchestrator behavior
grep -A 30 "TestFloorViolation\|TestShipWithoutBuild\|TestShipWithoutAudit\|TestFloorViolating" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/core/*.go 2>/dev/null | \
  grep -E "RunCycle\|RunPhase\|orchestrat\|Orchestrat" | head -5
```

### AC-4: Core statemachine spine tests still pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run "TestSpine\|TestFloor\|TestCan" -count=1 2>&1 | grep -E "ok|FAIL|---"
```

### AC-5: Router floor tests still pass (no regression) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/router/... -run "TestClamp\|TestFloor" -count=1 2>&1 | grep -E "ok|FAIL|---"
```

### AC-6: New test verifies ship is blocked when plan has ship=true, build=false, audit=false [code]
```bash
# The test description or comment must mention "floor-violating plan" or "skip build" or "skip audit"
grep -rn "floor.violat\|ship.*without.*build\|ship.*without.*audit\|skip.*build.*ship\|build.*false.*ship.*true" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/core/ \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/router/ 2>/dev/null | head -5
```

### AC-7: Full core package tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 2>&1 | grep -E "^ok|^FAIL" | tail -5
```
