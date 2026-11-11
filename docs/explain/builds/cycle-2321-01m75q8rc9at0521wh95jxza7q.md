# Build Explanation — Cycle 2321

## Build Binding
- Cycle: 2321
- Base SHA: 1eda02ad44b94999f572adfcfb07c454d69c3ad8

## Summary
Five producers now publish events to the channels. `evolve wave next` emits `wave.started` and `wave.ended`. `evolve ci watch` emits `ci.completed`. A red run is a WARN with the code `CI_RUN_RED`. The claim path emits `inbox.claimed`. The release paths emit `inbox.released`.

## Rationale
The closed sets of kinds and modules gate every event. The five kinds and the `ci` module are added there first. Each producer then emits through the existing Center, so the channel routes need no change. The `loop` and `signals` channels, and the `errors` channel for a WARN, receive the events.

## Changed Areas
- `go/internal/signalcenter/event.go` — adds the module `ci`, the five kinds and the code constant `CI_RUN_RED`.
- `go/internal/signalcenter/registry.go` — registers `CI_RUN_RED` with its documentation, so it exists in every build.
- `go/internal/signalcenter/event_test.go` — updates the closed-set counts and lists for the new names.
- `go/internal/ciwatch/ciwatch.go` — adds the `Workflow` and `Signals` options and emits `ci.completed` before any escalation.
- `go/cmd/evolve/cmd_ci_watch.go` — opens a Center for the role `ci` and names the workflow of each watched run.
- `go/internal/inboxmover/lifecycle_events.go` — adds the helpers that emit `inbox.claimed` and `inbox.released` as INFO events of the module `inbox`.
- `go/internal/inboxmover/claim_pending.go` — `ClaimDispatchable` emits `inbox.claimed` after a claim moves an item.
- `go/internal/inboxmover/claims.go` — `releaseHeld` emits `inbox.released` after a release ends a claim.
- `go/cmd/evolve/cmd_wave_next.go` — opens a Center for the role `wave`, emits `wave.started` after the launch and `wave.ended` after the closed record is saved.

## Design Decisions
A nil Center is the null object, so a caller without events needs no guard. A dry run, a usage error and a failed boundary emit nothing. The event of a run is emitted before the escalation, so a failed inbox write keeps the event. A refused claim and a release that moves nothing emit nothing. `ClaimDispatchable` publishes when its caller sets `Signals`. The host claimer and `ClaimPending` set it, but the `evolve inbox claim` CLI does not, so the CLI claim publishes nothing.

## Verification
Tests drive `runWaveWith`, `runCIWatch` and `ciwatch.Watch` through the production entry points. They read the channel files and assert the module, severity, code and fields.

## Compatibility
The legacy project stream keeps its allow-list, so the new kinds reach the channels only. The kind and module names are additive.

## Limitations
Only a red `ci.completed` event carries a code, `CI_RUN_RED`; the other events carry no code. The claim event has the cycle and the item id only.
