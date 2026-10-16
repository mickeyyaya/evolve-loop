# Comment history: `internal/phases/swarmrunner`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## groupKiller injected-kill fix

### `go/internal/phases/swarmrunner/kill_unix_test.go:41`

```text
// tmux teardown tests live in internal/swarm (ExecTmuxKill, seam-based — never
// a real tmux exec). The predecessor here live-fired `tmux kill-session -t ''`
// against the shared default socket, which tmux resolves to the CALLER'S OWN
// session — running this suite from inside any tmux pane killed that session
// (2026-06-11 killer-B). Do not reintroduce real-tmux calls in unit tests.
```

## comment-zero h2

### `go/internal/phases/swarmrunner/kill_unix_test.go:41`

```text
// tmux teardown tests live in internal/swarm (ExecTmuxKill, seam-based — never
// a real tmux exec). The predecessor here live-fired `tmux kill-session -t ''`
// against the shared default socket, which tmux resolves to the CALLER'S OWN
// session — running this suite from inside any tmux pane killed that session
// (2026-06-11 killer-B). Do not reintroduce real-tmux calls in unit tests.
```

### `go/internal/phases/swarmrunner/persona_forward_test.go:48` — above `func TestSwarmDecorator_ForwardsSignalsWired(t *testing.T) {`

```text
// ADR-0103 unit 11 (test 43): a decorator must not narrow the wiring proof of
// what it wraps — SignalsWired forwards to the inner runner, false for a
// runner without the capability.
```

### `go/internal/phases/swarmrunner/persona_forward_test.go:89` — above `func TestSwarmDecorator_ForwardsContractVerifierWired(t *testing.T) {`

```text
// Research F22: the verifier-wiring proof forwards through the Decorator the
// same way SignalsWired does — wrapping never reports a wired runner unwired,
// and never invents the capability for a runner without it.
```

### `go/internal/phases/swarmrunner/swarmrunner.go:1` — above `package swarmrunner`

```text
// Package swarmrunner is the orchestrator seam for the swarm harness (ADR-0032):
// a Decorator that wraps any core.PhaseRunner and, when the swarm stage is
// active and the planner produced a partitionable plan, dispatches the phase
// across N parallel workers instead of running it as a single agent.
//
// It is the ONE piece that turns the (otherwise inert) swarm building blocks
// into a live, orchestrator-managed multi-subagent phase. The orchestrator's
// dispatch call site is unchanged — it still calls runner.Run; this Decorator
// is wired in at the composition root (cmd/evolve/cmd_cycle.go) wrapping the
// build (writer) and scout (reader) runners.
//
// Rollout via Config.Stage (sourced from policy.json "swarm.stage"):
//   - off/shadow (default): delegate to the inner runner BYTE-FOR-BYTE. Zero risk.
//   - advisory: Dispatch runs LIVE (proving the parallel path + guaranteed
//     teardown) but the inner runner still produces the authoritative result;
//     swarm outcome is attached to Signals for inspection.
//   - enforce: the swarm result IS the phase output (writer: merge-train into the
//     integration branch; reader: synthesized summary).
//
// Safety: any missing/invalid/non-partitionable plan, or a writer plan that
// isn't provably disjoint, collapses to the inner single-writer runner (N=1).
```

### `go/internal/phases/swarmrunner/swarmrunner.go:51` — above `WorktreeBase string`

```text
// WorktreeBase is the resolved policy.json worktree.base ("" ⇒ default
// <root>/.evolve/worktrees), threaded to the worker provisioner. Replaces
// the former EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064).
```

### `go/internal/phases/swarmrunner/swarmrunner.go:308` — above `func (d *Decorator) SignalsWired() bool {`

```text
// SignalsWired forwards the verdict engine's root-wiring proof (ADR-0103 unit
// 11) to the decorated runner: a decorator must not narrow the capability of
// what it wraps, or the composition root's per-runner proof would read a
// wrapped scout/build as unwired. A wrapped runner without the method
// reports false.
```

### `go/internal/phases/swarmrunner/swarmrunner.go:320` — above `func (d *Decorator) ContractVerifierWired() bool {`

```text
// ContractVerifierWired forwards the verdict engine's verifier-wiring proof
// (research F22) the same way SignalsWired does.
```

### `go/internal/phases/swarmrunner/swarmrunner_test.go:588` — above `d := New(inner, &errBridge{}, swarm.ModeWriter, Config{Stage: "enforce", WorktreeBase: filepath.Join(root, ".evolve", "w…`

```text
// Exercise the Config.WorktreeBase DI seam (replaces the former
// EVOLVE_WORKTREE_BASE env read; flag-reduction, ADR-0064).
```
