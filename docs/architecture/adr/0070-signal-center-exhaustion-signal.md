# ADR-0070: Quota/rate-limit exhaustion as a first-class signal in the unified SignalCenter

**Status:** Accepted
**Date:** 2026-07-01
**Extends:** ADR-0068 (SignalCenter Facade), ADR-0047 (liveness Strategy / single-source-with-projection)

## Context / request

A tmux LLM-CLI that hits its quota/rate-limit **mid-phase** prints its error and
returns to the REPL prompt **without exiting a process code**. The phase wait
loop detected completion only by "did the artifact file appear?", and the
exit-code fallback (`llmroute` triggers `{80,81,85,124,127}`) and advisor
chain-walk (PR #292) both key on a process exit that never comes. Worse, the
driver's nudge-on-missing-artifact re-prompted the walled CLI, whose re-printed
error read as *new content* → `LivenessConverging` ("real output is never stuck")
→ the reviewer extended forever: a **livelock**. Agy/Gemini's "Individual quota
reached … Resets in 52h" wedged the router + behavior-baseline phases 15+ min.

The one pattern that matched the wall (`manifests/*.json` `usage.exhausted_regex`)
was consulted only by the usage probe at cycle start, never during phase
execution. The requested solution: detect this — and *all* CLI status —
**centrally**, in the unified SignalCenter, and dispatch it to registered
handlers through the existing abstractions.

## Decision

Model exhaustion as an **orthogonal, dominating** CLI-status signal in the
SignalCenter, using three design-pattern moves that build on ADR-0068's
Facade+Strategy without a switch edit:

1. **Projection (ADR-0047):** `PaneProfile.ExhaustedRegex`, populated by
   `paneProfileFor` from the manifest's `manifestExhaustedPattern` — the *same*
   maintained source the usage probe uses, so probe-time and phase-execution
   detection can never drift. `panestream` stays manifest-agnostic (it receives a
   regex string).

2. **Decorator:** `ExhaustionProbe` wraps *any* `LivenessProbe`. `Assess` always
   calls the inner probe (advancing its stateful `PaneDelta` cursor) then, if the
   pane matches `profile.ExhaustedRegex`, returns `LivenessExhausted` — overriding
   the inner verdict so a re-printed error can no longer masquerade as
   `Converging`. Empty/uncompilable pattern → transparent delegate (fail-open:
   never invents a wall). The center wraps every registered/default probe in one,
   so exhaustion flows through the same per-CLI abstraction as liveness.

3. **Observer:** `SignalCenter.RegisterSignalHandler(fn)` dispatches a
   `SignalEvent{SessionKey, State}` to registered handlers on each state
   *transition* (edge-triggered), outside all locks. This is the "detected and
   sent to the registered handler" path for reactive consumers (e.g. a future
   CLI-bench that benches a walled driver so later phases route around it).

`LivenessExhausted` is the top aggregate priority (a wall dominates). Consumers
use the fitting view of the one detected signal — no duplicated detection:
- **Wait loop** (`driver_tmux_repl.go`) already polls `Aggregate()`; on
  `LivenessExhausted` it returns `ExitUnknownPrompt (85)` → the dispatch chain
  fails over in one poll interval instead of burning the full artifact timeout.
- **Reactive consumers** register via `RegisterSignalHandler`.

### Alternatives rejected

- **Wire `ClassifyExhausted` directly into the wait loop** (point fix): would
  duplicate the detection outside the center and leave every other consumer
  blind. Rejected for centralization.
- **A new `rate_limit` regex per CLI in the interactive-prompt path**: the
  in-loop `autorespond` regexes already exist but (a) didn't match Gemini's
  wording and (b) are gated off by `paneBusy`. Single-sourcing the manifest
  `exhausted_regex` via the Decorator subsumes them.
- **A separate exhaustion subsystem**: violates "build on the unified center";
  exhaustion is just another CLI-status signal.

## Consequences

- A mid-phase wall fails over in ~one poll interval; the livelock is impossible
  (the override beats the growth-velocity "Converging" reading).
- Detection is single-source (manifest), per-CLI (Strategy/Projection),
  fail-open, and driver-agnostic (any CLI whose manifest defines the pattern).
- The Observer dispatch runs inline under `Observe` but outside all locks, so a
  slow/re-entrant handler never serializes other sessions or deadlocks; the
  ADR-0068 lock-ordering invariant is preserved.
- **Deferred (S3):** a production `RegisterSignalHandler` consumer that benches a
  walled CLI (the per-run center → cross-run `usageprobe.Store` seam). The
  dispatch mechanism is built + tested; the consumer wiring is a follow-up.

Files: `go/internal/bridge/panestream/{liveness,signalcenter,panedelta}.go`,
`go/internal/bridge/{tmux_pane_checks,driver_tmux_repl}.go`. Closes inbox
`driver-quota-hang-wedge` (S1–S2; S3 tracked).
