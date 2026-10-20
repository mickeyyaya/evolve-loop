# Build Explanation — Cycle 1999

## Build Binding
- Cycle: 1999
- Base SHA: 241e0782d293c8637bb4fc5b4a668eca1ab30a12

## Summary
`evolve routing plan --cycle N` asks the phase advisor for a plan from the recorded state of cycle N. It prints the plan and writes nothing under `runs/cycle-N`.

## Rationale
The advisor already composes its own prompt (`ComposePlanPrompt`). The command builds a `RouteInput` from the recorded cycle state and a scratch workspace, then calls `PhaseAdvisor.Plan`. Thus the prompt has one composer. The scratch workspace takes the advisor's capture artifacts, so the cycle directory stays unchanged.

## Changed Areas
- `go/cmd/evolve/cmd_routing_plan.go` — the new subcommand, the input builder `routingPlanInput` and the advisor seam `routingPlanAdvisor`. Exit codes: 1 unknown cycle, 2 advisor failure, 10 usage.
- `go/cmd/evolve/cmd_routing.go` — dispatches `plan`, lists it in the unknown-subcommand hint and shares `writePlanEntries` with `routing explain`.
- `go/cmd/evolve/registry.go` — the registry summary names the new capability.
- `go/internal/core/routing_dispatch.go` — `AdvisorCatalog` returns the phase cards and the on-demand names that the cycle gives the advisor.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the routing-plan tests that swap the `routingPlanAdvisor` seam as serial.

## Design Decisions
The advisor is wired through a package variable, so tests inject a fake bridge. The production wiring reuses `wireRouterAdvisor`, the same function the cycle uses. The rejected option is a second prompt builder in `cmd/evolve`: it would drift from the cycle prompt.

## Verification
`TestRoutingPlan_*` in `go/cmd/evolve/cmd_routing_plan_test.go` and `TestC1999_001` to `TestC1999_003` in `go/acs/cycle1999` pass. `evolve acs suite --cycle 1999` reports red=0.

## Compatibility
`routing explain` and `routing replay` print the same output as before. Only the usage text lists `plan`.

## Limitations
The command does not read the benched CLI families, the recalled lessons or the carryover todos. The plan can differ from the plan of a live cycle in those inputs.
