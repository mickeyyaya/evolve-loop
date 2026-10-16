# internal/reflection

## Purpose

`internal/reflection` is the Go port of the per-phase Reflection Journal YAML schema. It reads, writes and validates the sidecar `.evolve/runs/cycle-<N>/<phase>-reflection.yaml` and computes its canonical path. The enforcement half is `scripts/lifecycle/phase-gate.sh`. The consumer half is `scripts/observability/aggregate-reflections.sh`. Phase code and the aggregator use these types and do not reach back into bash. The schema reference is `agents/reflection-journal-schema.md`, and the shell fixture `scripts/tests/reflection-schema-test.sh` pins it.

## Design

- **`Sidecar(runsDir, cycle, phase)`** returns `<runsDir>/cycle-<N>/<phase>-reflection.yaml`, the path that the "reflection required" check of `phase-gate.sh` looks for.
- **`Write` validates first.** A malformed sidecar is worse than none, because the gate sees a present file, and the bug of the producer stays hidden. It then writes through a temporary file and a rename, so a concurrent reader never sees a half-written file. If the rename fails, the temporary file is removed.
- **`Parse` normalizes `slowdowns`.** yaml.v3 leaves `slowdowns: []` as nil. The parser makes it a non-nil empty slice. A caller can then tell "empty array, as the schema says" from "field absent". The aggregator depends on this when it emits `slowdown_categories: []`.
- **`Validate` checks what the YAML types cannot.** It checks the required fields: a cycle above zero, and a phase and agent that are not empty. It checks the severity and priority enums (`low`, `medium`, `high`). It checks that the confidence is in the range 0 to 1. It checks the schema version. A nil receiver returns an error and does not panic.
- **`SchemaVersionV1` is the only recognized version.** A later version is a validation error, so producers and consumers upgrade together.
- **`AcceptedByAggregator` is strict.** The aggregator rolls up a reflection only when the confidence is greater than `AggregatorConfidenceThreshold` (0.5). A value equal to it is rejected. The shell test T2 pins the same rule (0.2 skipped, 0.8 counted). The predicate exists so callers do not derive the threshold again.
- **The YAML tags are the wire contract.** Downstream consumers parse the sidecar, so the key names must not change.

## Invariants

- A schema version above 1 never validates (`TestSchemaVersionV1_GatesValidation`).
- A confidence equal to the threshold is not accepted (`TestAggregatorConfidenceThreshold_StrictBoundary`).
- `Write` refuses an invalid reflection and leaves no temporary file behind on a rename failure (`TestWrite_RefusesInvalid`, `TestWrite_RenameFailure_RemovesTmp`).
- An absent `slowdowns` field parses to an empty non-nil slice (`TestParse_AbsentSlowdownsNormalized`).

## Findings

- None recorded.
