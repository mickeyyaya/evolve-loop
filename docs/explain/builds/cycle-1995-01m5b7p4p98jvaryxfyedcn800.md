# Build Explanation — Cycle 1995

## Build Binding
- Cycle: 1995
- Base SHA: 2a8186caabc45c83889e1dc6aeb3c172c54f9f41

## Summary
The pool admission has an optimistic rule behind a new policy key. At the default `shadow` stage the admitted set stays as before, and each todo that only the import-reach rule holds is recorded with its holder.

## Rationale
Waves 130 to 133 showed a core lane holding 13 ready todos for 71 minutes because core imports most packages. Two todos conflict in fact only when they edit one package or both are hub todos. A shadow stage first measures the gain without a change of behavior.

## Changed Areas
- `go/internal/admission/admission.go` — adds `Optimistic`, `OnShadow` and `ShadowAdmission`; the optimistic rule is the shared-edit test alone.
- `go/internal/orchestrator/orchestrator.go` — registers the signal `LOOP_ORCH_ADMISSION_SHADOW`.
- `go/internal/orchestrator/pool.go` — adds `RecordShadow`, which publishes a shadow admission.
- `go/internal/policy/fleet_orchestrator.go` — adds the `admission` key, default `shadow`, with strict validation.
- `go/cmd/evolve/cmd_loop_orchestrator.go` — `cellAdmission` selects the rule from the key and wires the shadow sink.
- `docs/architecture/signal-codes.md` — regenerated for the new code.
- `go/internal/policy/fleet_orchestrator_test.go` — existing expectations now carry the default `Admission: "shadow"`.
- `docs/architecture/packages/internal-admission.md` — documents the rule and the record.
- `docs/architecture/packages/internal-orchestrator.md` — documents `RecordShadow`.
- `docs/architecture/packages/internal-policy.md` — documents the key.

## Design Decisions
The shadow check reuses the one conflict function split in two (`sharedEdit`, `conflict`), so the new rule cannot drift from the old one. The key is separate from the orchestrator stage so the two can move independently.

## Verification
Unit, property (500 random backlogs) and ACS tests cover the three acceptance criteria and the production caller `cellAdmission`.

## Compatibility
Without the key the admitted set is byte-identical to before. Only a new INFO signal appears.

## Limitations
The optimistic rule does not govern until an operator sets `admission` to `enforce`.
