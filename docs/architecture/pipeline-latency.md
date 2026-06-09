# Pipeline Latency — where a cycle's wall time goes, and how to cut it

> Design: [ADR-0043](adr/0043-pipeline-latency-program.md).
> Sibling workstream (test latency, separate): `go/docs/testing.md` §"known fast-suite poles".

This is the **production**-latency companion to the test-latency pass. Test latency is what a
developer waits for; *pipeline* latency is what every autonomous cycle pays, on every phase, on
every run. This doc maps where that wall time goes (grounded in the dispatch code, not guesswork),
evaluates the three candidate levers, and recommends a measured, risk-ranked sequence.

## Why this matters

A cycle dispatches a chain of LLM phases — `intent → scout → triage → … → build → audit → ship →
retrospective` (15 registered phases; routing runs a subset, typically ~8–10 LLM dispatches). Each
dispatch that goes through a tmux-REPL driver (`claude-tmux`, `codex-tmux`, … — the default path)
pays a **cold-boot tax** before the model does any work. That tax is paid N times per cycle and is
pure overhead.

## Where the time goes (dispatch anatomy)

Per-phase dispatch path (`go/internal/bridge/driver_tmux_repl.go`, ephemeral-session branch):

| Step | Cost | Source |
|---|---|---|
| `tmux new-session` | process spawn | `driver_tmux_repl.go:162` |
| **fixed sleep** | **1s hardcoded** | `driver_tmux_repl.go:165` (`deps.Sleep(time.Second)`) |
| `cd <workdir>` | one SendKeys | `:166` |
| **fixed sleep** | **1s hardcoded** | `driver_tmux_repl.go:167` |
| launch CLI + marker poll | `bootIntervalS` × ticks-until-prompt-marker, 60s deadline | `:185-210` |

So **cold boot ≈ 2s fixed + bootInterval×ticks** per phase, *before the prompt is even delivered*.
`bootIntervalS`: claude-tmux=1, codex-tmux=2, agy-tmux=2, ollama=1 (`driver_claudetmux.go:61` et al);
boot deadline `tmuxREPLBootTimeoutS=60` (`:63`). Optimistically 1–2 ticks to the marker →
**~3–5s/phase**; × ~8 dispatches → **~25–40s/cycle of pure boot overhead**, independent of model
think time. The interactive ("human cadence") path adds *more* deliberate latency
(`human_input.go:43` `humanBootPause`, `:51` `humanPasteWithReview`).

Crucially, a **warm path already exists**: when `BridgeRequest.SessionName != ""` the driver
resolves a *named* session and **skips the entire boot block** (`driver_tmux_repl.go:161`
`if !namedExists`). Today only the **swarm harness** sets `SessionName` (ADR-0032); ordinary serial
phases pass it empty → ephemeral → **cold boot every single phase**. The machinery to avoid the tax
is built; the serial pipeline just doesn't use it.

## The three levers, evaluated against the code

### Lever A — REPL boot reuse  ★ recommended (highest leverage)
Cold boot is paid per phase and the avoidance machinery (named sessions) already exists. Two
sub-levers, smallest-first:

- **A1 — adaptive boot wait (low-risk first win).** Replace the two unconditional `Sleep(1s)` with
  a *poll-until-ready* (poll the pane for the shell prompt before `cd`, and for cd-return before
  launch) with the current fixed sleep as the timeout fallback. Removes up to ~2s/phase with a
  bounded blast radius (one driver function, fallback preserves today's behavior). **Gate on A0
  measurement** (boot waits exist for real tmux/shell readiness; tune, don't delete).
- **A2 — pre-warmed session pool (bigger win).** Keep a small pool of *already-booted* REPLs per
  CLI; a phase grabs a warm one instead of cold-booting. **Hard constraint: this is pooling, NOT
  cross-phase session sharing.** Reusing one live REPL across phases would let `build` inherit
  `scout`'s context and would break the trust-kernel's per-phase isolation (builder≠auditor,
  cross-family floor — see CLAUDE.md). A pooled session must be **context-cleared** (`/clear` or
  fresh process) and re-marker-confirmed before handing to the next phase. Pre-warm happens in the
  background during the *previous* phase's think time, so the boot tax overlaps useful work.

### Lever B — prompt-cache-aware ordering  ✅ already implemented
The adapter (`go/internal/adapters/bridge/bridge.go:120-128`) already assembles prompts
cache-first: `Correction > Rules > Policy > Contract-invariant-block > Body > volatile-path-footer`
— stable/cacheable content as the prefix, the per-cycle artifact path in the **last line**. The
deterministic fan-out cache-prefix (`go/internal/subagent/cacheprefix.go`) reinforces this for
swarm workers. **The ordering work is done.** The only open question is *empirical*: does each
tmux-driven CLI actually realize the Anthropic prompt cache across separate REPL sessions (the cache
is keyed on exact prefix bytes, 5-min TTL)? That is a measurement, not a code change — and it
becomes moot under A2 (a warm pooled session keeps the CLI's own conversation cache hot anyway).

### Lever C — swarm read-parallelism  (high ceiling, high cost — defer)
Read-only phases (scout, audit) could fan out / overlap instead of running strictly serially. The
swarm harness already runs workers under a bounded semaphore (`EVOLVE_SWARM_CONCURRENCY`, default 2)
but is stage-gated **off** (`EVOLVE_SWARM_STAGE=shadow`). Real ceiling on read-heavy cycles, but it
needs advisor+orchestrator coordination and careful gate semantics — larger and riskier than A.
Revisit after A lands and the measurement shows residual serial read cost.

## Recommended sequence (measured, risk-ranked)

0. **A0 — instrument boot_ms (do first; it's the measurement gate).** Add an additive `BootMS`
   field threaded `bridge driver → core.BridgeResponse → core.PhaseResponse → phaseTimingEntry`
   (omitempty, backward-compatible). The boot loop (`driver_tmux_repl.go:190-210`) already knows
   the exact boot window; surface its duration up the in-process call chain (the bridge is
   in-process — `engine.go` drives tmux via `exec.CommandContext`, so no cross-process channel is
   needed). Exact seam map in the next section. Behavior-neutral → safe to ship alone; one real
   measured cycle then tells us boot-vs-think split per phase.
1. **A1 — adaptive boot wait**, behind a default-off stage flag, validated against A0 numbers.
2. **A2 — pre-warmed pool**, behind the same flag family (`off|shadow|enforce`, centralized config —
   no flag sprawl), once A0/A1 prove the boot share is worth the pool's complexity.
3. **C — swarm read-parallelism**, only if measurement shows meaningful residual serial read cost.

## A0 instrumentation seam (for whoever implements it)

In-process flow already carries `DurationMS` the same way; `BootMS` rides alongside it:

- `go/internal/core/ports.go:318` — `BridgeResponse`: add `BootMS int64 \`json:"boot_ms,omitempty"\``.
- `go/internal/bridge` (`engine.go` + `driver_tmux_repl.go`) — capture `start` at
  `:161` (just before `NewSession`) and stamp elapsed at `promptSeen` (`:206`); populate
  `BridgeResponse.BootMS`. Named/warm path → `BootMS=0` (no boot), which is itself the signal.
- `go/internal/phases/runner/runner.go` — copy `bres.BootMS` onto every `PhaseResponse` it builds
  (~5 sites near `:498-566`).
- `go/internal/core/orchestrator.go:977` — `phaseTimingEntry`: add `BootMS int64
  \`json:"boot_ms,omitempty"\``; copy at the two construction sites (`:1168`, `:2082`).
- Unit-test with the bridge package's existing seam-injection style (fake `Tmux`/`Sleep` in `Deps`),
  asserting `BootMS` reflects the simulated marker-poll window and is `0` on the named-session path.

## Risks & non-goals

- **Phase isolation is sacred.** No lever may let one phase's REPL context leak into another.
  A2 pools *fresh/cleared* sessions; it never shares a live conversation across phases.
- **Boot waits guard real readiness.** A1 tunes them adaptively with a fallback; it does not delete
  them blindly (a too-eager launch races the shell/CLI and reintroduces the `exit 80` boot flakes
  already documented for real-tmux tests).
- **Measure before changing the hot loop.** A0 ships first and gates A1/A2. A blind sleep cut across
  every phase/CLI is exactly the kind of change that breaks every cycle at once.
- **Not in scope:** test-suite latency (separate workstream; `go/docs/testing.md`).
