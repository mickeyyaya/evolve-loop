# How to add a pipeline phase

> A *phase* is one stage of the cycle lifecycle: it dispatches a persona via the
> bridge, reads one contracted artifact, and emits a verdict. Adding one is a
> self-registering package + a persona + a state-machine edge. This guide walks
> the real moving parts using the **debugger** phase (`go/internal/phases/debugger/`)
> as the worked example — it is the newest phase and exercises every seam.

Read first: [architecture/phase-pipeline.md](../architecture/phase-pipeline.md) §2–§3
(the phase graph + the `BaseRunner` Template-Method contract).

## The shape of a phase

Every subagent-dispatching phase reuses one skeleton — `runner.BaseRunner`
(`go/internal/phases/runner/runner.go`). You supply a tiny `Hooks` value; the base
runner does profile lookup → prompt composition → bridge dispatch → artifact read →
classification → `PhaseResponse` packaging. You write ~5 variation points, not ~70
lines of boilerplate.

## Step 1 — the `core.Phase` constant

Add your phase identity to `go/internal/core/phase.go`: a `Phase` const and a case
in `Phase.IsValid()`. (Debugger is `PhaseDebugger Phase = "debugger"`.) Phases are
stringly-backed for JSON portability; the verdict vocabulary
(`VerdictPASS/FAIL/WARN/SKIPPED`) is fixed — do not invent a fifth.

## Step 2 — the state-machine edges

Wire the legal transitions in `go/internal/core/statemachine.go`:

- Add `from → to` entries to the `allowed` map in `NewStateMachine()`. Debugger is
  reachable from ship (`PhaseShip: {…PhaseDebugger: true…}`) and routes onward to
  `{PhaseShip, PhaseAudit, PhaseBuild, PhaseTDD, PhaseEnd}`.
- If the successor is **deterministic on the verdict**, add a case to `Next()`. If
  it is **decision-driven** (the phase emits a choice the orchestrator reads), leave
  `Next()` returning `PhaseEnd` and let the orchestrator override via `scheduledNext`
  — this is exactly how `retro` and `debugger` work (see phase-pipeline §2/§5).

`CanTransition` re-validates every chosen edge, so an illegal edge fails loudly
rather than silently mis-routing.

## Step 3 — the `Hooks` implementation

In `go/internal/phases/<name>/<name>.go`, implement the five `runner.Hooks` methods
(`runner.go:Hooks`). From debugger:

```go
func (hooks) PhaseName() string                           { return string(core.PhaseDebugger) }
func (hooks) AgentPromptName() string                     { return "evolve-debugger" } // loads agents/evolve-debugger.md
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "debug-decision.json" }
func (hooks) DefaultModel() string                        { return "opus" }
func (hooks) ComposePrompt(body string, req core.PhaseRequest) string { /* append cycle context */ }
func (hooks) Classify(_ string, req core.PhaseRequest, _ core.BridgeResponse) (verdict string, diags []core.Diagnostic, nextPhase string) { /* read the artifact, map to a verdict */ }
```

Notes grounded in the real code:

- **The profile name is the agent name with `evolve-` stripped** — `runner.go` does
  `strings.TrimPrefix(AgentPromptName(), "evolve-")` to find
  `.evolve/profiles/<agent>.json` and the `EVOLVE_<AGENT>_*` env keys. Name your
  persona `evolve-<name>.md` and your profile `<name>.json` accordingly.
- **`Classify` runs only on the success branch.** `BaseRunner.Run` handles the
  bridge-error and missing-artifact paths before calling it. Keep `Classify` a
  *pure* read of the artifact (debugger's is a standalone `Classify(dir)` function so
  it is unit-testable without the bridge) and **safe-default loudly** — debugger maps
  any missing/malformed/unknown decision to BLOCK, never RESHIP.

## Step 4 — optional behaviors (only if you need them)

- **Skippable** — implement `runner.Skipper` (`ShouldSkip`) to short-circuit before
  any bridge call. Used by triage (`EVOLVE_TRIAGE_DISABLE`) and tdd
  (`EVOLVE_TEST_PHASE_ENABLED=0`).
- **Optional (non-essential)** — pass `Optional: true` to `runner.New` so an artifact
  timeout degrades to WARN and the cycle advances instead of aborting. Build-planner
  does this (`buildplanner.go`: `runner.Options{… Optional: true}`).
- **Emitting signals** — `BaseRunner` does *not* populate `PhaseResponse.Signals`
  from `Classify`. If the orchestrator must read a cross-phase decision, wrap the base
  `Run` and enrich `resp.Signals` yourself — debugger's `Phase.Run` does exactly this
  to surface `debugger.action`/`debugger.rerun_phase`.
- **Source-writing** — if the phase writes code (not just a report), it must run in
  the worktree. Built-in `tdd`/`build` are recognized by `worktreePhase`; a
  spec-driven phase sets `writes_source`. The role-gate denies writes anywhere else.

## Step 5 — self-register

At the bottom of your package, register the factory so the dispatcher finds it with
no switch-statement edit (Factory Method, `phases/registry`):

```go
func init() {
	registry.Register(string(core.PhaseDebugger), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{Bridge: bridge.NewDefault(req.ProjectRoot), Prompts: prompts.NewForProject(req.ProjectRoot)})
	})
}
```

## Step 6 — the persona

Write `agents/evolve-<name>.md` with the YAML front-matter (`name`, `model`,
`capabilities`, `tools`) and a body that states the contract precisely: **where to
write the artifact** (the workspace root — never a `workspace/` subdir, see
[architecture/cli-matrix-and-drivers.md](../architecture/cli-matrix-and-drivers.md)
fact #1), the exact filename, and the schema. `agents/evolve-debugger.md` is the model.

## The tests that pin this

- `go/internal/phases/debugger/debugger_test.go` — `TestClassify`,
  `TestClassifyNeverReshipOnParseFailure` (the safe-default invariant).
- `go/internal/phases/runner/runner_test.go` — `TestRun_HappyPath_DelegatesToHooksAndBridge`,
  `TestRun_BridgeError_ReturnsFAILWithDiagnostic`, `TestRun_ArtifactFileFallback`.
- `go/internal/phases/runner/runner_optional_test.go` —
  `TestRun_OptionalPhase_ArtifactTimeout_DegradesToWarn`,
  `TestRun_MandatoryPhase_ArtifactTimeout_StillFails`.
- `go/test/trustkernel/trustkernel_test.go` —
  `TestStateMachine_AuditVerdictRoutesShipOrRetro`,
  `TestStateMachine_ShipFollowsAuditOnlyViaShippableVerdict` (your edges must not
  break the spine).
