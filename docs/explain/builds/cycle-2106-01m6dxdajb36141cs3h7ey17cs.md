# Build Explanation — Cycle 2106

## Build Binding
- Cycle: 2106
- Base SHA: d81eb9540fbe0ddef32072bcf1f896be3e49409e

## Summary
The doctor deep probe now runs through the optional `DeepProber` driver interface. The probe argv carries the manifest fast model for agy.

## Rationale
Previously `doctor.go` hardcoded LLM argument vectors and ignored manifest models. Probe construction through drivers ensures model consistency across CLI families.

## Changed Areas
- `go/internal/bridge/doctor.go` — delegates deep probe execution to driver `DeepProber` and returns failed `DeepProbe` with `Err` on manifest load failure to block the verdict.
- `go/internal/bridge/driver.go` — declares `DeepProber` interface, implements `DeepProber` for `claudePDriver`, and maps bare family `ollama` to `ollama-tmux` in `bareDriverMap`.
- `go/internal/bridge/driver_agy.go` — implements `DeepProber` by delegation to shared `headlessProbeArgs` in `driver.go` to supply manifest fast model.
- `go/internal/bridge/driver_codex.go` — implements `DeepProber` to provide the `exec` probe arguments.

## Design Decisions
The `DeepProber` interface defines `DeepProbeArgs(m Manifest) []string`. Drivers that do not implement `DeepProber` skip the deep probe check. The bare family `ollama` resolves to `ollama-tmux` in `bareDriverMap`. A manifest load failure returns a failed `DeepProbe` with `Err` to block the verdict.

## Verification
Targeted unit tests and ACS cycle predicates pass. Guard tests verify that `doctor.go` contains no hardcoded LLM argv literals.

## Compatibility
`DriverFor` retains compatibility for existing callers. The `DeepProber` interface is optional and does not break existing drivers.

## Limitations
This cycle does not remove `DriverFor` because frozen ACS predicates call `DriverFor`. Removal requires console intervention.
