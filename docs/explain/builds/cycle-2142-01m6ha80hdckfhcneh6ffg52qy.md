# Build Explanation — Cycle 2142

## Build Binding
- Cycle: 2142
- Base SHA: eb57adbbe5c5a700e6a2b52994b8ee7770be8153

## Summary
The audit repair now calls the convergence policy after each audit FAIL. The default stage is shadow. The decision is recorded as a CONVERGENCE_RUNG signal, and the repair flow does not change.

## Rationale
The convergence package decides the next rung of a repeat-until-accepted loop, but no pipeline loop called it. An adapter turns the audit rounds into judgments. It is the smallest step that lets the repair record a decision before it follows one.

## Changed Areas
- `go/internal/core/audit_repair_convergence.go` — builds judgments from the archived and live audit reports, calls the decision, and emits the signal.
- `go/internal/core/audit_fail_decision.go` — consults the adapter before the grant or decline. An enforced Stop turns a granted repair into a decline.
- `go/internal/policy/workflow.go` — carries the resolved convergence config in the workflow config.
- `go/internal/signalcenter/event.go` — adds the convergence module that owns the CONVERGENCE_RUNG code.
- `go/internal/signalcenter/event_test.go` — updates the closed module set from 36 to 37.
- `go/internal/core/audit_repair_decision_signal_test.go` — ignores the new rung signal when it counts the repair signals.
- `go/internal/core/audit_repair_convergence_untitled_test.go` — tests that an untitled finding is keyed by its report id.
- `go/internal/core/audit_repair_convergence_test.go` — pins the adapter, the shadow flow, the enforce flow and the replay.
- `go/acs/cycle2142/predicates_test.go` — the acceptance predicates of the cycle.
- `.evolve/evals/convergence-v5-audit-repair-decide.md` — the eval graders of the task.
- `docs/architecture/signal-codes.md` — regenerated from the code registry.
- `docs/architecture/packages/internal-core.md` — records the new invariant of the audit repair.

## Design Decisions
Only a Stop is followed at the enforce stage. Land, Split and the other actions leave the envelope's grant, because an audit FAIL with no parsed finding must not lose its repair. A Continue never lifts an envelope decline. The adapter keys a finding by its title, because report ids restart in each round.

## Verification
The new core tests, the policy and signal center suites and the five cycle predicates pass. Each function of the adapter has 100% statement coverage.

## Compatibility
The default stage is shadow, so the repair flow is unchanged. A workflow config with no convergence block consults nothing.

## Limitations
The adapter passes no fixer, judge or headroom table to the decision yet. The judge raise and the fresh-context fixer appear in the replay test, not in a live dispatch.
