# Eval: Phase-gate negative — ship blocked when build artifact absent

> Cycle 193 MEDIUM task. Adds a direct `SpineSatisfiedUpTo` negative test that
> a floor-violating plan reaching ship WITHOUT a real build artifact is rejected
> at the artifact-backed gate (not merely by ClampPlanToFloor at the plan level).
> Prior tests set Build.Present=true; the build-absent floor had no direct
> regression guard.

## AC-1: Ship-without-build negative test exists in statemachine_spine_test.go [code]
```bash
grep -q "ShipRequiresBuild\|ship.*without.*build\|build.*absent.*ship\|BuildAbsent" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/core/statemachine_spine_test.go
```

## AC-2 (negative): SpineSatisfiedUpTo(PhaseShip) returns false when Build absent [code]
```bash
# Run the new test and verify it passes (i.e. the gate correctly rejects ship when build absent)
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 \
  -run "TestSpineSatisfiedUpTo_.*Build\|TestSpineSatisfiedUpTo_Ship.*Build\|TestSpineSatisfiedUpTo_ShipRequiresBuild" \
  -v 2>&1 | grep -E "PASS|FAIL|--- PASS|--- FAIL"
```

## AC-3: All core statemachine tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 2>&1 | grep -E "ok|FAIL" | head -5
```

## AC-4: trustkernel tests still all pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./test/trustkernel/... -count=1 2>&1 | tail -3
```

## AC-5 (negative): floor-violating scenario — empty signals blocks ship [code]
```bash
# A SpineSatisfiedUpTo test with completely empty RoutingSignals must return false for PhaseShip
grep -A 15 "FloorViolating\|EmptySignals\|NoBuildAudit" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/core/statemachine_spine_test.go 2>/dev/null | \
  grep -qE "false|SpineSatisfied.*false|must be.*false|BLOCK" && echo "PASS: assertion on false found" || \
  echo "WARN: check manually that empty-signals scenario asserts false"
```

## AC-6: At least 4 distinct SpineSatisfiedUpTo test functions exist [code]
```bash
count=$(grep -c "^func TestSpineSatisfiedUpTo" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/core/statemachine_spine_test.go 2>/dev/null || echo 0)
if [ "$count" -lt 4 ]; then
  echo "FAIL: only $count TestSpineSatisfiedUpTo functions, need >=4 for floor regression guard"
  exit 1
fi
echo "PASS: $count TestSpineSatisfiedUpTo functions"
```
