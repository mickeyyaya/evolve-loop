# Observer False-Positive Stall on Long tmux Turns — Root Cause + Fix

**Date:** 2026-06-02
**Found in:** cycle-190 (TDD phase), during the `harden` test-resilience run.
**Fix branch:** `fix/observer-tmux-liveness`

## Symptom

`tdd-observer-events.ndjson` for cycle-190:
```
12:05:24 started      (observer attached)
12:15:29 stall_no_output  INCIDENT  "no stdout growth for 10m0s"
12:18:18 stopped      "context canceled"
```
But the TDD agent was **alive the whole time** — the tmux scrollback showed
`Incubating… (12m 50s · ↑ 54.0k tokens)`, it wrote `test-report.md` (170 lines),
and the cycle advanced normally to `build`. The `stopped: context canceled` was
the orchestrator's normal phase teardown, **not** a kill (the auto-spawn path is
observe-only). So: a **false-positive INCIDENT**, harmless this time, but a
**latent wrongful-kill** if enforce is ever enabled on the auto-spawn path.

## Root cause

There are two observer implementations:
- `internal/phaseobserver` — standalone `evolve phase-observer` (has `Enforce`/`KillPgrp`, emits `stuck_no_output`).
- `internal/adapters/observer` — the **auto-spawn `CoreAdapter`** used by `evolve loop` (observe-only, emits `stall_no_output`). **This one fired.**

The auto-spawn observer (`observer.go:Watch`) resets its stall clock on exactly
two signals:
1. **stdout-log size growth**, and
2. **newest workspace-file mtime** (the cycle-141 tmux fix: `WorkspaceDir`).

A `claude-tmux`/`agy-tmux`/`codex-tmux` agent in a **long single "Incubating"
turn** (extended thinking + one large tool call) commits **no scrollback lines**
to the stdout-log and writes **no workspace artifact** until the turn ends —
then dumps everything at once. For minutes, *both* filesystem signals are flat,
indistinguishable from a hang. The only liveness signal in that window is the
**live tmux pane** (spinner / token-counter advancing every second), which the
observer never consulted.

## Fix (cycle-190)

Add an injectable `LivenessProbe func() bool` to the observer `Config` (DI seam;
nil = byte-identical legacy behavior). The `Watch` loop consults it **only at
the stall threshold**, before emitting the incident: on `true`, it resets the
clock and emits a benign `stall_probe_active` info event instead of a false
`stall_no_output`. Concrete probe (`tmux_probe.go`) locates the bridge session
by the deterministic infix `-c<cycle>-<phase>-` (sessions are named
`evolve-bridge-<cli>-c<cycle>-<phase>-pid<pid>-<ts>`) and hashes `capture-pane
-p`; a changed hash (or first sighting) ⇒ pane animating ⇒ agent alive. No
matching session / tmux absent / capture error ⇒ `false` (no claim → legacy
stall logic proceeds). Wired default-on for every phase at the `CoreAdapter`
(non-tmux phases find no session ⇒ no-op). **No new env flag** (avoids flag
sprawl).

## Design notes

- The probe runs at most once per `StallS` window, so the `tmux capture-pane`
  subprocess never costs anything on the common healthy path.
- This is the research-backed *probe-before-trip* circuit-breaker pattern
  (see `ai-factory-pipeline-resilience-2026-06-02.md`, idea #2/#5): a liveness
  check should not trip on a single blunt signal (byte growth) when a richer
  positive signal (pane animation) is cheaply available.
- Tests: `observer_test.go` (`TestWatch_LivenessProbeSuppressesFalseStall`,
  `TestWatch_LivenessProbeFalseStillStalls`) + `tmux_probe_test.go` (session
  match / no-match / animating / frozen / errors) — all with an injected
  command runner, no real tmux required.

## Follow-up (deferred)

- The **standalone** `internal/phaseobserver` (the one that *does* kill) has the
  same byte-only blind spot. Port the same `LivenessProbe` seam there so an
  ENFORCE run can't wrongfully SIGTERM a long-turn agent. Tracked for a
  follow-up cycle.
