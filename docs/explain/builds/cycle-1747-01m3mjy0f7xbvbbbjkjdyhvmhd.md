# Build Explanation — Cycle 1747

## Build Binding
- Cycle: 1747
- Base SHA: 292d11bed29bc4d0c73bf51b3bcee505a66d20fe

## Summary
The `run_dir.artifact_bytes` scout signal returns as a typed field, `router.ScoutSignals.ArtifactBytes`.
`router.Digest` reads it only from the scout handoff's top-level `run_dir.artifact_bytes` key. The
routingtest fixture `SignalSpec.ArtifactBytes` emits the same value through both renderings, the pure
`Signals()` and the on-disk `HandoffFiles()`/`WrappedHandoffFiles()`, so the dual-rendering keystone
covers the signal.

## Rationale
Cycle 1250 injected `Generic["run_dir.artifact_bytes"] = dirSize(workspace)` inside `Digest`. The pure
rendering cannot model a directory size computed at runtime, so the routingtest keystone
(`TestSignalSpec_DualRenderingAgree`) became unsatisfiable and main stayed red until the revert. This
build makes the value data that the scout handoff carries. Both renderings can then emit it from one
fixture field, and parity holds by construction. The field is typed rather than a Generic key, which
matches how every other scout signal (`BacklogSize`, `CarryoverCount`) is extracted.

## Changed Areas
- `go/internal/router/signals.go` — adds `ArtifactBytes int` to `ScoutSignals`, next to the other scout counters.
- `go/internal/router/digest.go` — `extractScout` unmarshals `top["run_dir.artifact_bytes"]` into `ScoutSignals.ArtifactBytes`. A missing, null or non-numeric value leaves 0 (fail-open, like its neighbours). Nothing is computed from the run directory, and nothing is written to `Generic`.
- `go/internal/routingtest/spec.go` — adds the `SignalSpec.ArtifactBytes` fixture field.
- `go/internal/routingtest/render.go` — `scoutPresent()` counts `ArtifactBytes > 0`, and `Signals()` copies the field into `ScoutSignals`. The scout handoff map moved out of `HandoffFiles()` into a new `scoutHandoff()` helper, which writes the `run_dir.artifact_bytes` key when it is non-zero. The move keeps `HandoffFiles()` inside the function-size ratchet: the new key alone would have grown it to 63 lines, over its 60-line allowance. The wrapped renderer inherits the key through `HandoffFiles()`.
- `go/internal/sizeratchet/offenders.json` — drops the `routingtest.SignalSpec.HandoffFiles` allowance. After the extraction the function fits the default 50-line limit, so the ratchet now holds it there.
- `go/internal/routingtest/consistency_test.go` — the TDD phase added ArtifactBytes rows to both keystone tables, plus a non-vacuity test showing the value survives both renderings.
- `go/acs/cycle1747/predicates_test.go` — the TDD phase's acceptance predicates: flat and wrapped parity, handoff-only extraction against a 256 KiB run-dir decoy, fail-open, package and regression-selection guards, and (C008) a router probe checking that this document names the `context-condense` `insert_when` consumer the typed field cannot reach.
- `.evolve/evals/artifact-bytes-signal-dual-rendering.md` — the TDD phase's score-cap eval for the six criteria above.

## Design Decisions
The on-disk key is the literal `run_dir.artifact_bytes` at the top level of `handoff-scout.json`, so
the name matches the reverted signal. The value lives on `ScoutSignals`, not `RoutingSignals.Generic`,
and that keeps the cycle-1250 shape impossible to re-land silently: predicates C003 and C004 assert
that the Generic key is absent. A non-zero value alone makes scout present in the fixture, the same
rule the other scout fields follow, so the two renderings stay in lock-step for an
`{ArtifactBytes: N}`-only fixture.

The signal does have a consumer. The `context-condense` phase gates itself with the `insert_when`
condition `run_dir.artifact_bytes gt 102400` (`.evolve/phases/context-condense/phase.json:31`). Routing
conditions resolve through `resolveField` in `go/internal/router/condition.go`, and `resolveField` has no
case for `ScoutSignals.ArtifactBytes`. The field name therefore falls through to `resolveGeneric`
(`condition.go:104-105`), which reads `RoutingSignals.Generic`, and this build leaves that plane empty on
purpose. So even when a handoff carries 200000 bytes, the typed field is invisible to the
`context-condense` `insert_when` and the phase does not fire. Adding a typed `resolveField` case for
`run_dir.artifact_bytes` was kept out of this build: `condition.go` is outside this item's file scope,
and the queued item `insert-when-fields-need-a-producer` owns producer validation for exactly this
trigger.

## Verification
`go test -count=1 ./internal/router/ ./internal/routingtest/ ./internal/core/... ./internal/phaseio/... ./internal/regressiontia/`
all pass, and so does the whole module: `go test -count=1 ./...` reports 246 ok packages, and the size ratchet is green. `go test -tags acs ./acs/cycle1747` passes 8/8; C008 probes the router and confirms that a 200000-byte handoff leaves the `context-condense` `insert_when` unfired through the typed field. The keystone tests
`TestSignalSpec_DualRenderingAgree`, `TestSignalSpec_WrappedDualRenderingAgree` and
`TestSignalSpec_ArtifactBytesReachesBothRenderings` pass. `evolve acs suite --cycle 1747` returns PASS with red=0.

## Compatibility
The change is additive: a new struct field and a new optional JSON key. Existing handoffs without the key
digest exactly as before (ArtifactBytes=0). No exported function signature changes.

## Limitations
The signal is inert in production for two separate reasons, and fixing one does not fix the other:

1. No live writer puts the real run-directory size into the scout handoff yet. That writer is outside
   this item's file scope and is deferred, so the field stays 0 until a scout-side writer lands.
2. Even with a writer, routing cannot read the typed field. `resolveField` has no case for it, so the
   `context-condense` `insert_when` on `run_dir.artifact_bytes` still resolves through the empty Generic
   plane and never fires. Predicates C003 and C004 forbid the Generic injection, so the consumer side needs
   a typed `resolveField` case. That work belongs to the queued item `insert-when-fields-need-a-producer`.
