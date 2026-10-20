---
score_cap:
  - criterion: "evolve routing plan --cycle N prints the parsed plan from a fake bridge and writes nothing under runs/cycle-N"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestRoutingPlan_PrintsParsedPlanAndWritesNothingUnderTheCycle' ./cmd/evolve"
  - criterion: "the prompt routing plan sends equals the advisor composer's prompt for the same input"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestRoutingPlan_SendsTheCyclesOwnComposedPrompt' ./cmd/evolve"
  - criterion: "an unknown cycle exits 1 and never launches the advisor"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestRoutingPlan_UnknownCycleExitsOneWithoutLaunchingTheAdvisor' ./cmd/evolve"
  - criterion: "an advisor launch failure exits 2 and prints the cause"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestRoutingPlan_AdvisorLaunchFailureExitsTwoWithTheCause' ./cmd/evolve"
  - criterion: "the built binary knows routing plan and exits 1 for an unknown cycle"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1999_001' ./acs/cycle1999"
---

# Eval: evolve routing plan --cycle N (lane part)

> Pins the lane half of the `cli-routing-plan` request: `evolve routing plan`
> asks the phase advisor for a plan on demand, from cycle N's recorded state,
> without writing into the cycle. The prompt comes from the one advisor
> composer, and the exit codes are 0, 1 for an unknown cycle and 2 for a
> launch failure. Source: inbox item `cli-routing-plan-lane`, cycle 1999.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| plan-printed-nothing-written | fake-bridge plan is printed; runs/cycle-N is unchanged | 5/10 | `go test -run TestRoutingPlan_PrintsParsedPlanAndWritesNothingUnderTheCycle ./cmd/evolve` |
| one-composer | sent prompt equals the composer output for the same input | 6/10 | `go test -run TestRoutingPlan_SendsTheCyclesOwnComposedPrompt ./cmd/evolve` |
| unknown-cycle | exit 1, advisor never launched | 6/10 | `go test -run TestRoutingPlan_UnknownCycleExitsOneWithoutLaunchingTheAdvisor ./cmd/evolve` |
| launch-failure | exit 2, cause on stderr | 6/10 | `go test -run TestRoutingPlan_AdvisorLaunchFailureExitsTwoWithTheCause ./cmd/evolve` |
| binary-reachability | built binary dispatches routing plan | 5/10 | `go test -tags acs -run TestC1999_001 ./acs/cycle1999` |
