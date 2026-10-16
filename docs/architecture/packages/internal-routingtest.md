# internal/routingtest

## Purpose

`internal/routingtest` is the composable test framework for the dynamic phase-routing kernel. A scenario is data (`ScenarioSpec`). Bricks (`bricks.go`) assemble it, or `Matrix` (`matrix.go`) generates it. The engine (`engine.go`) runs each spec against a chosen `Surface` and checks the expectations that the spec carries.

A new test is a new config. A new combination is a composition of existing bricks, or a new matrix dimension.

## Design

- **A plain package that only tests import.** It is not a `_test` package, so `router_test` and `core_test` can import it. It imports `core`, `router` and `config`. Only `*_test.go` files import it, so it adds no production import cycle.
- **Two surfaces, one fixture.** `PureKernel` makes one `router.Route()` decision with no I/O. `FullOrchestrator` runs `core.Orchestrator.RunCycle` from start to end.
- **A `SignalSpec` renders two ways.** `Signals()` serves the pure kernel. `HandoffFiles()` and `WrappedHandoffFiles()` (the ADR-0050 phase-3 payload envelope) serve the `Digest` of the orchestrator. A role is present when any of its fields is set. Only present roles emit a file, as fail-open does.
- **The scripted agent stands in for the LLM brain.** `scriptedProposer` is keyed by the phase that just completed. It returns a canned proposal or a forced error, which tests the fall back to the static path. A nil proposal for a phase is legal, and the kernel decision stays as it was.
- **One agent backs both router ports.** The same instance is the Proposer and the Planner, as the single PhaseAdvisor of production is.
- **The scripted plan is unclamped, and the engine clamps it.** The agent returns the plan as scripted. The engine runs `ClampPlanToFloor` before it passes the plan on, as the orchestrator does (ADR-0024). A plan that reaches ship without build and audit is completed by the integrity floor and never stays weak.
- **The pure surface passes a plan only when the spec scripts one.** Without a plan, or with a forced plan error, `in.Plan` stays nil and the legacy trigger path runs.
- **The option order matters on the orchestrator surface.** `WithRouting` comes first, because a later `WithPlanner` lands on a struct whose strategy is already set. `WithPlanner` is added only when a plan is scripted. The enables of the spec also go into the phase enables of the workflow config. The intent-gate logic of the orchestrator then sees the same enables as the router.
- **Branch reasons match by containment.** The disposition gate adds a loud prefix to the reason when a fixture delivers no `disposition.json`. The test asserts the branch marker with `Contains`, not `HasPrefix`.
- **Bricks never alias memory of the caller.** `Mandatory` and `Done` copy their variadic slices. If they kept the backing array, a caller that reuses its slice can corrupt the spec.
- **Defaults match production.** `buildConfig` fills the safe defaults that `config.Load` fills: stage off, the default spine, and the tester trigger `build.acs_red>0`. `SeverityTrigger` swaps the trigger to `build.severity_max>=HIGH`. `Strict` mirrors the `workflow.strict_audit` posture. A seeded failure has a `RecordedAt` far in the future, so it stays in its retention window.
- **Cadence is hybrid.** `ExpectProposeAt` asserts the exact set of phases where the proposer ran. With an upfront plan, `Propose` fires only at branch transitions (ADR-0024).

## Invariants

- **The model proposes and the kernel disposes.** `invariantChecks` run over the adversarial cross-product. A proposal never changes the `NextPhase` that the kernel chose, however adversarial it is. `applyProposal` only records a clamp.
- **A clamped plan that runs ship also runs build and audit.** The check reads the plan as the engine passed it on. It holds whatever the advisor proposed and however small the mandatory list is.
- **The two renderings agree.** `TestSignalSpec_DualRenderingAgree` pins the flat form. `TestSignalSpec_WrappedDualRenderingAgree` pins the wrapped envelope for every routing fixture. `TestSignalSpec_ArtifactBytesReachesBothRenderings` proves that the agreement is not empty, because both renderings can drop a field and still agree.
- **Bricks copy their slices.** `TestBricks_VariadicSlicesAreDefensivelyCopied` pins it.
- **A matrix yields the product of its dimensions, and each spec has a unique name.** `TestMatrix_CrossProductCount` and `TestDimension_ExpandsInMatrix` pin it.
- **The orchestrator surface runs the full spine.** `TestFullOrchestrator_SelectsCycleSurface` pins it.
- **Failed-record specs seed the retro input.** `TestFailedRecordSpec_SeedsFailedApproaches` pins it.
- **Each unmet expectation of the pure surface is reported once.** `TestCheckPureExpectations_ReportsEachMismatchedExpectation` pins it. A matching decision reports nothing.

## Findings

- None.
