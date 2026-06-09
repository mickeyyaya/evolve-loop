# ADR-0043: Pipeline-latency program — attack cold REPL boot, measurement-gated

> Status: **A0 Implemented** (`12dcd18b`, tracked binary rebuilt `aacd95de`; 2026-06-09). **A1/A2/C remain
> Proposed**, gated on A0 measurement + a default-off stage flag + explicit operator greenlight. Design-first:
> this ADR records the analysis and chosen *direction*; hot-loop code does not ship on assertion. Full analysis:
> [pipeline-latency.md](../pipeline-latency.md). Sibling (test latency, separate & already resolved):
> `go/docs/testing.md`. **First measurement + as-built notes: see §"A0 — as-built + first measurement" below.**

## Context

The `/evolve` perf goal targets both test latency and **pipeline** latency. Test-suite latency was
handled separately (core/observer parallelized in `9724461d`; ship-test parallelization proven a
dead-end — `go/docs/testing.md`). This ADR covers the production side: the wall time every
autonomous cycle pays dispatching its LLM phases.

Grounding the cost in the dispatch code (`go/internal/bridge/driver_tmux_repl.go`): each phase that
uses a tmux-REPL driver (the default) cold-boots a fresh tmux session — **2s of hardcoded
`Sleep(1s)` (`:165`,`:167`) + a `bootIntervalS`×ticks marker-poll** — *before* the prompt is even
delivered. Paid ~8–10× per cycle → an estimated ~25–40s/cycle of pure boot overhead. A warm path
(named sessions, `:161` `if !namedExists`) already exists but is used only by the swarm harness;
serial phases pass an empty `SessionName` and cold-boot every time.

Two assumptions from the initial scoping were corrected by reading the code:
- **Prompt-cache-aware ordering is already implemented** (`internal/adapters/bridge/bridge.go:120-128`):
  stable/cacheable content is the prefix, the volatile per-cycle path is the last line. No work needed.
- **The bridge is in-process** (`engine.go` drives tmux via `exec.CommandContext`), so boot timing
  can be threaded up the call chain without a cross-process channel.

## Decision

Pursue **REPL boot reuse** as the primary lever, measurement-gated and risk-ranked:

| Step | What | Risk | Gate |
|---|---|---|---|
| **A0** | Instrument `BootMS` (additive field, driver→`BridgeResponse`→`PhaseResponse`→`phaseTimingEntry`) | behavior-neutral | ship alone; produces the boot-vs-think split |
| **A1** | Adaptive boot wait — poll-until-ready replacing the 2 fixed sleeps, fixed sleep as fallback | bounded (one driver fn) | gated on A0 numbers + default-off flag |
| **A2** | Pre-warmed session **pool** (NOT cross-phase sharing) — context-cleared + re-marker-confirmed per phase | higher | gated on A0/A1; default-off stage flag |
| **C** | Swarm read-parallelism for read-only phases (scout/audit) | high | only if A leaves residual serial read cost |

Lever **B (prompt-cache ordering) is already done** — the remaining question is empirical (does the
tmux CLI realize the API cache across sessions) and becomes moot under A2.

## A0 — as-built + first measurement (2026-06-09)

**As-built (deliberately diverged from the proposed seam).** The proposed seam captured wall-clock
`start→promptSeen`. As shipped, the driver instead **counts intended sleep/poll iterations**
(`bootWaitMS = fixedReadinessWaits×1000 + bootInterval×polls`, `driver_tmux_repl.go:196-200`) and fires an
injected `deps.OnBoot(bootWaitMS)` callback **once** on the cold-boot marker (`:227`). Rationale:
iteration-counting is **deterministic under test** (where `Sleep` is a no-op) yet ≈ wall-clock in prod — which
let the 3 unit tests assert exact values (cold=3000ms, boot-timeout=0, warm named-session=0). `OnBoot` never
fires on the warm (`namedExists`) path or a boot timeout, so `boot_ms=0` is itself the "no cold boot" signal.
The value rides the in-process chain `driver → BridgeResponse → PhaseResponse → phaseTimingEntry`; the deferred
`phase-timing.json` write persists it even when the cycle errors.

**First measured cycle (262).** `phase-timing.json`:

| phase | duration | boot_ms | boot share |
|---|---|---|---|
| scout (claude/sonnet) | 218.2s | 6000ms | 2.7% |
| tdd (claude/opus) | 253.7s | 3000ms | 1.2% |
| retro (claude/auto) | 1213.6s (FAIL) | 0 | — |

**Finding 1 — boot is a small fraction of think-heavy phases.** The 6s/3s boot matches the per-phase design
estimate (~3–5s), but as a *fraction* of multi-minute LLM phases it is only ~1–3%. So the A1/A2 boot-reduction
levers have **bounded payoff for think-heavy phases**; they matter most for short, high-count phase sequences.
This *lowers* A1/A2 priority below the design's initial framing — exactly the correction the measurement gate
exists to produce. A1/A2 stay Proposed; revisit only if a cycle's phase mix is demonstrably boot-bound.

**Finding 2 — A0 coverage gap (ties to ADR-0044).** Only 3 of 5 phases recorded a timing entry: **build and
audit are absent.** Build went through the **CLI-fallback** path (codex→claude), which does not write a
`phase-timing` entry (the same un-reconciled-fallback defect as [ADR-0044](0044-unified-phase-recovery-protocol.md)
D1); audit never ran (the cycle mis-recorded build as failed and skipped it). And retro shows `boot_ms=0` despite
reaching the `❯` prompt — its claude booted into a fatal `--model auto` error (ADR-0044 D3), a degenerate boot
the cold-boot accounting doesn't attribute. **A0 is correct on the primary-dispatch happy path but has coverage
holes on fallback / failed-phase paths; closing ADR-0044 C1 (single-source reconciliation) also closes the
timing gap.**

## Consequences

- **Positive:** removes overhead paid on every phase of every cycle, using machinery (named sessions)
  that already exists. A0 is a clean, safe, immediately-shippable measurement foundation.
- **Constraint (non-negotiable):** phase isolation. A2 pools *fresh/cleared* REPLs; it must never
  share a live conversation across phases (that would break builder≠auditor / cross-family floor).
- **Constraint:** boot waits guard real tmux/shell readiness — A1 tunes adaptively with a fallback,
  it does not delete them (a too-eager launch reintroduces `exit 80` boot flakes).
- **Discipline:** no hot-loop change ships without A0 measurement first. A blind sleep cut across
  every phase/CLI is precisely the change that breaks every cycle at once.
- **No flag sprawl:** A1/A2 share one centralized `off|shadow|enforce` stage knob, mirroring
  `EVOLVE_SWARM_STAGE`.

## Alternatives considered

- **Cut the fixed sleeps outright (no poll, no flag).** Rejected: unmeasured, breaks readiness
  guarantees across CLIs, blast radius = every cycle.
- **Share one REPL across all phases.** Rejected: destroys per-phase trust isolation.
- **Lead with swarm read-parallelism (C).** Deferred: higher coordination cost and gate-semantics
  risk than A, for a win that only materializes on read-heavy cycles.
- **Re-order prompts for cache.** Already done (Lever B); no action.
