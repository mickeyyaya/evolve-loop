# Eval: add-plan-e2e-test

## Graders

```bash
# G1: Check that orchestrator_plan_e2e_test.go file exists
ls go/internal/core/orchestrator_plan_e2e_test.go
```

```bash
# G2: Check that TestOrchestrator_Advisory_EmitsPhasePlanAndRoutingDecisions is defined in orchestrator_plan_e2e_test.go
grep -q "func TestOrchestrator_Advisory_EmitsPhasePlanAndRoutingDecisions" go/internal/core/orchestrator_plan_e2e_test.go
```

```bash
# G3: Check that config.StageAdvisory is used in orchestrator_plan_e2e_test.go
grep -q "config.StageAdvisory" go/internal/core/orchestrator_plan_e2e_test.go
```

```bash
# G4: Check that phase-plan.json is asserted in orchestrator_plan_e2e_test.go
grep -q "phase-plan.json" go/internal/core/orchestrator_plan_e2e_test.go
```

```bash
# G5: Check that routing-decision is asserted in orchestrator_plan_e2e_test.go
grep -q "routing-decision" go/internal/core/orchestrator_plan_e2e_test.go
```

```bash
# G6: Run the newly created Go test and ensure it passes successfully
cd go && go test -v ./internal/core -run TestOrchestrator_Advisory_EmitsPhasePlanAndRoutingDecisions
```
