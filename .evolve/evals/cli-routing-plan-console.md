---
score_cap:
  - criterion: "routing plan prints the parsed plan against a fake bridge and writes nothing under runs/cycle-N"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRoutingPlan_PrintsParsedPlanAndWritesNothingUnderTheCycle$' ./cmd/evolve"
  - criterion: "routing plan sends the prompt the cycle composer builds for the same input"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRoutingPlan_SendsTheCyclesOwnComposedPrompt$' ./cmd/evolve"
  - criterion: "an unknown cycle exits 1 and an advisor launch failure exits 2 with the cause"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestRoutingPlan_UnknownCycleExitsOneWithoutLaunchingTheAdvisor|TestRoutingPlan_AdvisorLaunchFailureExitsTwoWithTheCause)$' ./cmd/evolve"
  - criterion: "runtime-reference.md documents evolve routing plan with its flags and exit codes"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2140_004 ./acs/cycle2140"
---

# Eval: evolve routing plan, console part

> Pins the three acceptance criteria of inbox item cli-routing-plan-console and
> the operator documentation its fix names. The command shipped in cycle 1999;
> cycle 2140 found the three behaviors already green and the runtime-reference
> entry missing.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| plan-prints-and-isolates | fake-bridge plan, cycle dir untouched | 7/10 | `TestRoutingPlan_PrintsParsedPlanAndWritesNothingUnderTheCycle` |
| one-composer | sent prompt equals composer output | 7/10 | `TestRoutingPlan_SendsTheCyclesOwnComposedPrompt` |
| exit-codes | exit 1 unknown cycle, exit 2 launch failure | 7/10 | `TestRoutingPlan_UnknownCycleExitsOneWithoutLaunchingTheAdvisor` |
| operator-docs | runtime-reference entry | 6/10 | `TestC2140_004` |
