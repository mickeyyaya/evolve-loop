# ADR-0030: Phase-Observer Auto-Spawn in `evolve loop`

**Status:** Accepted | **Date:** 2026-05-28 | **PR:** _pending (cycle-122 remediation, commit 4)_ | **Supersedes:** N/A | **Builds on:** [ADR-0029 CLI Fallback Chain](./0029-cli-fallback-chain-and-per-agent-overrides.md), [ADR-0027 commit-as-evidence](./0027-commit-as-evidence.md)

---

## Context

The `evolve loop` autonomous dispatcher has **no per-phase stall detector wired in**. The phase-observer code exists at `go/internal/phaseobserver/phaseobserver.go` (feature-complete: tracks tool_use/tool_result events, supports `EVOLVE_OBSERVER_STALL_S` / `MaxNoProgressS` / `EVOLVE_OBSERVER_NUDGE_S`) and is invokable as the standalone CLI subcommand `evolve phase-observer`. But the autonomous orchestrator at `go/internal/core/orchestrator.go:RunCycle` never spawns it.

**This is a silent regression from the v12.0.0 flag day.** The pre-v12 bash dispatcher (`archive/legacy/scripts/dispatch/run-cycle.sh:726-732`) unconditionally background-spawned `phase-observer.sh` with `--enforce --scope=cycle` whenever `EVOLVE_OBSERVER_ENFORCE=1` (default-on since v10.18.0). The Go port preserved the observer **code** and shipped it as a manual subcommand, but never re-added the **auto-spawn** at orchestrator startup. The [CLAUDE.md env-var table](../../../CLAUDE.md) reads `EVOLVE_OBSERVER_ENFORCE=1 default-on since v10.18.0` — that documentation is technically true for the bash dispatcher and the standalone subcommand, but **factually false for the modern Go `evolve loop` path**.

**Cycle 122 is the first run where this silent regression actually bit.** Codex-tmux at the tdd phase blocked on a CLI-native permission modal for ~10 minutes before the bridge's coarse artifact-timeout (10 min budget) fired with `exit=81`. WS-G's fallback chain (ADR-0029) didn't recover because 81 wasn't in its default trigger list (`[80, 127]`). Had the phase-observer been auto-spawned, it would have detected the file-never-created stall at the 90 s grace mark and SIGTERMed the subagent, putting WS-G's chain in a position to retry on claude-tmux or agy-tmux. Full forensics: [cycle-122 incident report](../../incidents/cycle-122-codex-permission-modal-and-wsg-fallback-gap.md).

Two cousin features have the same composition-root gap:

- **WS-E1 `MaxNoProgressS`** (babbling-livelock detector, shipped in PR #25) — defined but never wired (`cmd_phase_observer.go:64-79` does not pass the field; defaults to 0 = disabled). The orchestrator never constructs the observer at all, so even if wired, it wouldn't run.
- **WS-E2 `DeliverableReviewer.WithReviewer`** (per-phase output reviewer, shipped in PR #25) — interface exists; `cmd_cycle.go:wireOrchestratorDeps` (lines 295-304) never calls `WithReviewer()`, so `noopReviewer{}` (auto-approve) is the runtime behavior.

This ADR addresses the **orchestrator-side auto-spawn** of the phase-observer (the cycle-122 trigger). The WS-E1/E2 wiring is a follow-up that benefits from the same composition-root edits.

## Decision

**Auto-spawn the phase-observer goroutine from the orchestrator's `RunCycle` for every phase, gated by a single rollout env var.**

### Wiring (composition root)

1. **`go/internal/core/orchestrator.go`** — add a new `Observer` interface field on the Orchestrator struct (alongside the existing `reviewer DeliverableReviewer` field). Default it to a `noopObserver{}` whose `Start(ctx, PhaseRequest) (cancel func())` returns a no-op cancel function. Provide a `WithObserver(Observer) Option` functional option that follows the existing `WithReviewer` pattern.

2. **`RunCycle`** — before invoking each phase's `runner.Run(...)`, call `cancel := o.observer.Start(ctx, req)` and `defer cancel()`. The observer goroutine inherits the cycle context and is canceled when the phase completes (success OR failure). The orchestrator does NOT wait on the observer; if the observer SIGTERMs the subagent, the runner sees the exit code via its normal bridge return path.

3. **`go/cmd/evolve/cmd_cycle.go:wireOrchestratorDeps`** — when `os.Getenv("EVOLVE_OBSERVER_AUTOSPAWN") != "0"`, wire `WithObserver(phaseobserver.Adapter{...})`. The adapter is a thin wrapper that translates `core.PhaseRequest` → `phaseobserver.Config` and starts the existing `phaseobserver.Run` goroutine.

4. **`go/internal/phaseobserver/phaseobserver.go`** — add one new config field, `FileNeverCreatedGraceS int` (default 90s, read from `EVOLVE_OBSERVER_GRACE_S`). In `tail()` (current line 333-335 silently returns nil on missing file), record the phase start time and emit `stuck_no_output` INCIDENT + SIGTERM when `elapsed since phase start > FileNeverCreatedGraceS` AND the stdout-log file still doesn't exist. This is the new defense layer that cycle-122 needed (its tdd-stdout.log never appeared).

### Rollout flag

| Env var | Default | Effect |
|---|---|---|
| `EVOLVE_OBSERVER_AUTOSPAWN` | `1` (auto-spawn on) | When `1` (or unset), orchestrator wires a real `phaseobserver.Adapter`. When `0`, orchestrator uses `noopObserver{}` — byte-identical behavior to the pre-ADR-0030 autonomous loop, the rollback hatch. |
| `EVOLVE_OBSERVER_ENFORCE` | `1` (kill on stall) | Existing semantics preserved. Used by the spawned observer to decide whether to SIGTERM on stall (`1`) or just log INCIDENTs (`0`). Now reachable from the autonomous loop (previously only reachable from the manual subcommand). |
| `EVOLVE_OBSERVER_STALL_S` | `600` | Existing semantics preserved. |
| `EVOLVE_OBSERVER_GRACE_S` | `90` (NEW) | File-never-created grace: from phase start, if `<workspace>/<agent>-stdout.log` doesn't appear within this many seconds, emit `stuck_no_output` + SIGTERM. Cycle-122's exact shape. |
| `EVOLVE_OBSERVER_NUDGE_S` | `0` (off) | Existing semantics preserved. When `>0` and `NUDGE_S <= idle < STALL_S`, soft-stall nudge is appended to the agent inbox per ADR-0023. |
| `EVOLVE_OBSERVER_NUDGE_BODY` | (existing default) | Existing semantics preserved. |

The CLAUDE.md env-var table is updated to truthfully reflect that `EVOLVE_OBSERVER_AUTOSPAWN=1` is the default and the observer **is** auto-spawned by `evolve loop` (today the table is misleading).

## Considered Alternatives

### Alt A — Subprocess `os/exec` `evolve phase-observer` per phase (rejected)

> *Use `exec.Command("evolve", "phase-observer", "--enforce", ...)` to fork the existing CLI subcommand as a separate process per phase.*

**Why rejected:** Three problems.
1. **Binary path resolution complexity** — the orchestrator would need to know its own binary path (works for `go/bin/evolve` but not for ad-hoc test binaries like the cycle-122 `/tmp/evolve-g2-hardened`); the in-process goroutine avoids this entirely.
2. **Lifetime management** — a separate process needs explicit kill on phase end + zombie reaping; the in-process goroutine inherits the cycle context and cancels naturally.
3. **Observability cost** — separate process means separate stdout/stderr streams to capture; in-process events flow through the orchestrator's existing log infrastructure.

The legacy bash dispatcher did spawn a subprocess (because it WAS bash, no goroutines), but the Go port can do strictly better.

### Alt B — Bridge-layer stall detector instead of orchestrator-layer (rejected)

> *Move stall detection into the bridge driver so each driver can detect its own stall.*

**Why rejected:** Wrong layer.
1. The bridge driver doesn't know phase semantics (artifact expectations, deliverable timing); the orchestrator does.
2. Each driver would need duplicated stall logic, then they'd drift.
3. The phase-observer's "file never created" check needs to know the expected output path — that's an orchestrator concern.

The bridge already has its `core.ErrArtifactTimeout` as a coarse 10-min ceiling; the observer is the finer-grained per-phase signal that complements (not replaces) the bridge backstop.

### Alt C — Shadow → advisory → enforce rollout (rejected by operator decision)

> *Three-stage rollout per the ADR-0024 PhaseAdvisor pattern: shadow logs what would happen, advisory acts but only warns, enforce kills.*

**Why rejected (per operator decision):** The behavior change here is **restoring** a pre-v12 default-on behavior that has been baked under the manual subcommand for many versions. The failure mode the rollout would observe (silent hang) doesn't produce shadow-comparable output anyway — there's no "would have killed at X time" signal to log without actually starting the observer. The simple rollback hatch (`EVOLVE_OBSERVER_AUTOSPAWN=0`) is sufficient.

### Alt D — Wire WS-E1 `MaxNoProgressS` and WS-E2 `WithReviewer` in this same ADR (rejected)

> *Since the composition-root edit is open anyway, wire all three at once.*

**Why rejected:** Different failure modes. WS-E1 catches **babbling livelock** (agent keeps emitting assistant_text but never tool_use); WS-E2 catches **bad deliverables** (artifact produced but fails some quality check). Neither helps cycle-122 (the failure mode is "no events at all, ever"). Bundling them inflates the PR diff and the rollout risk. They get their own follow-up ADRs (or amendment of this ADR) when they ship.

## Consequences

### Positive

- **The cycle-122 failure shape is structurally defended** — codex modal would be killed at minute 1:30 instead of minute 10, putting WS-G's fallback chain (with Fix 2 from the same remediation extending triggers to include `exit=81`) in position to retry on another CLI.
- **CLAUDE.md becomes truthful** — `EVOLVE_OBSERVER_ENFORCE=1 default-on` actually reaches the autonomous loop.
- **WS-E1/E2 wiring becomes a small follow-up** — the composition root now has an observer field; wiring `MaxNoProgressS` from env or `WithReviewer` from a profile field is a one-line addition.
- **Restores pre-v12 behavior** — no policy surprise for operators who knew the bash dispatcher's auto-spawn.
- **Per-phase ceiling without a separate `--max-duration` flag** — the F5 deferred follow-up (dispatcher wall-clock timeout) is subsumed because every phase gets a stall ceiling.

### Negative

- **One more goroutine per phase** — negligible cost (the observer is event-driven; idle CPU is near-zero).
- **`EVOLVE_OBSERVER_AUTOSPAWN=0` rollback path must be tested** — the noop adapter must produce byte-identical orchestrator behavior. The cycle-122 commit-4 verification asserts this.
- **CLAUDE.md table requires churn** — multiple rows touch observer behavior; future env-var changes need to update all of them together.

### Neutral

- **The manual `evolve phase-observer` subcommand stays** — operators who want to attach an observer to a custom (non-`evolve loop`) workflow keep that path. It now shares config-resolution code with the orchestrator's auto-spawn (refactor it into `phaseobserver.ConfigFromEnv()`).
- **Existing `noopReviewer` pattern** at `go/internal/core/orchestrator.go:175-186` is the proven template for the new `noopObserver`.

## Implementation Notes

### Why a noop default, not a nil pointer

Following the existing `noopReviewer{}` precedent: nil-check at every observer call site is more code than a small noop struct. The noop pattern also lets `EVOLVE_OBSERVER_AUTOSPAWN=0` produce exactly the same call path as `EVOLVE_OBSERVER_AUTOSPAWN=1` with a no-op adapter — easier to reason about than two distinct code paths.

### Why phase start, not bridge launch, as the grace-timer epoch

The grace timer (`FileNeverCreatedGraceS`) measures from when `runner.Run` enters the phase, NOT from when the bridge subprocess starts. Reason: bridge boot can legitimately take 30-60s (REPL warmup, model first-token latency); measuring from bridge launch would set the grace window too late. Phase start is the deterministic anchor: the moment the orchestrator commits to running this phase.

### Why SIGTERM, not SIGKILL, on stall

SIGTERM gives the subagent a chance to flush any in-flight writes (the tmux driver, for instance, may have a final scrollback capture pending). SIGKILL is reserved for the bridge's outer hard-timeout (the existing `core.ErrArtifactTimeout` path). The observer fires first and softer; the bridge fires later and harder.

### Why the new field on `phaseobserver.Config`, not a new env var only

The new `FileNeverCreatedGraceS` is a first-class observer behavior, not a one-off flag. Same-shape additions in the future (e.g., a `MaxStdoutSizeBytes` to detect a runaway-output livelock) should follow the same config-field-then-env-binding pattern, not stack env vars on the side.

### Forward compatibility

When new CLIs are added, they integrate with the observer automatically — the observer watches files in the workspace path (not bridge-driver-specific paths). The driver-agnostic file-watch approach makes ADR-0030 robust to ADR-0029's "any CLI for any phase" expansion: no new wiring per CLI.

## Validation

**Unit + integration tests (commit 4 of cycle-122 remediation):**

- `go test ./internal/phaseobserver -run TestFileNeverCreatedGrace` — asserts grace timer fires after 90 s of file absence, emits `stuck_no_output` event, signals SIGTERM.
- `go test ./internal/core -run TestOrchestrator_AutoSpawnsObserver` — asserts `WithObserver` wires the goroutine; cancel function called on phase end; noop default produces zero observer events when `EVOLVE_OBSERVER_AUTOSPAWN=0`.
- `go test ./internal/phases/runner -run TestFallback_TriggersOnObserverSignal` — integration: observer SIGTERMs subagent → runner receives `ExitArtifactTimeout` (81) → WS-G chain retries on next CLI per Fix 2.

**Live verification:** cycle 123 (post all four commits) re-runs the cycle-122 multi-CLI fan-out. Pass criterion: `ps -eo` shows the observer goroutine per phase; if any phase blocks before producing stdout, an `observer-events.ndjson` entry appears within 90 s with `stuck_no_output`; the cycle reaches `audit` or beyond.

## References

- **Plan:** `~/.claude/plans/iterative-chasing-thunder.md`
- **Cycle-122 incident report:** [../../incidents/cycle-122-codex-permission-modal-and-wsg-fallback-gap.md](../../incidents/cycle-122-codex-permission-modal-and-wsg-fallback-gap.md)
- **Cycle-121 incident report:** [../../incidents/cycle-121-codex-repl-boot-timeout-and-ws-g-multi-cli.md](../../incidents/cycle-121-codex-repl-boot-timeout-and-ws-g-multi-cli.md)
- **ADR-0029 CLI fallback chain (the WS-G doc this ADR pairs with):** [./0029-cli-fallback-chain-and-per-agent-overrides.md](./0029-cli-fallback-chain-and-per-agent-overrides.md)
- **ADR-0023 live-injection and launch rules:** [./0023-live-injection-and-launch-rules.md](./0023-live-injection-and-launch-rules.md) (NudgeS soft-stall pattern referenced here)
- **Pre-v12 bash dispatcher reference:** `archive/legacy/scripts/dispatch/run-cycle.sh:726-732` (the unconditional background-spawn this ADR restores in the Go port)
- **Phase-observer implementation:** `go/internal/phaseobserver/phaseobserver.go`
- **Manual subcommand entrypoint:** `go/cmd/evolve/cmd_phase_observer.go`
