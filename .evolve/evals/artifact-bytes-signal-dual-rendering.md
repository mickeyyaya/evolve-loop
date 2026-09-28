---
score_cap:
  - criterion: "The keystone parity tests carry ArtifactBytes fixture rows and pass: Signals() equals router.Digest over HandoffFiles() and WrappedHandoffFiles()"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestSignalSpec_(Wrapped)?DualRenderingAgree$' ./internal/routingtest/"
  - criterion: "The fixture's ArtifactBytes value reaches both renderings (non-zero, scout present), so parity is not satisfied vacuously by both sides dropping it"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestSignalSpec_ArtifactBytesReachesBothRenderings$' ./internal/routingtest/"
  - criterion: "router.Digest reads Scout.ArtifactBytes from handoff-scout.json key run_dir.artifact_bytes (flat and wrapped), never from the run directory's size"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1747_00[123]_' ./acs/cycle1747/"
  - criterion: "An absent, non-numeric or null run_dir.artifact_bytes fails open to 0 and no run_dir.artifact_bytes key is injected into RoutingSignals.Generic"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1747_004_AbsentOrCorruptArtifactBytesFailsOpen$' ./acs/cycle1747/"
  - criterion: "A router change widens the per-cycle regression scope to ./internal/routingtest/..."
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestChangedScope_WidensByReverseDependency$' ./internal/regressiontia/"
  - criterion: "The router and routingtest packages pass"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/router/ && go test -count=1 ./internal/routingtest/"
  - criterion: "The build explanation names the registry's insert_when consumer of run_dir.artifact_bytes (context-condense), never claims no consumer asks for it, and, while no typed resolveField case reaches that consumer, discloses the gap in Design Decisions and the queued insert-when-fields-need-a-producer item in Limitations"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1747_008_' ./acs/cycle1747/"
---

# Eval: run_dir.artifact_bytes scout signal through the dual-rendering path

> Pins the sanctioned re-land of the `run_dir.artifact_bytes` telemetry
> signal. Cycle 1250 (99a82b3c, reverted 2026-08-03) injected
> `sig.Generic["run_dir.artifact_bytes"] = dirSize(workspace)` straight into
> `router.Digest`. The pure `SignalSpec.Signals()` rendering cannot model a
> runtime-computed directory size, so routingtest's keystone
> (`TestSignalSpec_DualRenderingAgree`) became unsatisfiable and main stayed
> red for 5 commits. The re-land (cycle 1747) makes the value a fixture field:
> `SignalSpec.ArtifactBytes` feeds both `Signals()` and `HandoffFiles()`, and
> `router.Digest` extracts it as `ScoutSignals.ArtifactBytes` from the scout
> handoff. This eval caps the audit score when the parity, the non-vacuity of
> that parity, the handoff-only extraction, or the fail-open edge regresses.
> Writing the real value into the live scout handoff is deferred (outside this
> item's `files` scope).
>
> Cycle 1747 audit round 1 (H1) failed the build explanation: it said no
> consumer asks for the signal, but context-condense's `insert_when`
> (`.evolve/phases/context-condense/phase.json`) routes on
> `run_dir.artifact_bytes` through `resolveField`'s Generic fallback, which the
> typed `ScoutSignals.ArtifactBytes` never reaches. The last cap pins an
> explanation that states that reach accurately.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| keystone-parity | ArtifactBytes rows in both keystone tables pass | 8/10 | `go test -run '^TestSignalSpec_(Wrapped)?DualRenderingAgree$' ./internal/routingtest/` |
| non-vacuous-parity | value survives both renderings | 8/10 | `go test -run '^TestSignalSpec_ArtifactBytesReachesBothRenderings$' ./internal/routingtest/` |
| handoff-not-dirsize | Digest reads the handoff key, flat and wrapped | 7/10 | `go test -tags acs -run '^TestC1747_00[123]_' ./acs/cycle1747/` |
| fail-open-no-generic | bad/absent value → 0, no Generic injection | 7/10 | `go test -tags acs -run '^TestC1747_004_' ./acs/cycle1747/` |
| regression-selection | router change selects routingtest | 5/10 | `go test -run '^TestChangedScope_WidensByReverseDependency$' ./internal/regressiontia/` |
| package-suites | router + routingtest green | 6/10 | `go test ./internal/router/ && go test ./internal/routingtest/` |
| explanation-reach | explanation names the insert_when consumer and the resolveField gap | 6/10 | `go test -tags acs -run '^TestC1747_008_' ./acs/cycle1747/` |
