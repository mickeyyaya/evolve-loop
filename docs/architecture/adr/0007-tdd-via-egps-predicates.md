# ADR-7: TDD via EGPS Predicates

**Status:** Accepted  
**Date:** 2026-05-15  
**Cycle:** 60 (documenting methodology shipped across cycles 40–60)  
**Implemented in:** `acs/cycle-N/*.sh`, `scripts/verification/validate-predicate.sh`,
                    `scripts/lifecycle/run-acs-suite.sh`

---

## Context

Cycles 102–111 demonstrated indirect reward hacking via confidence-cliff calibration: auditor prose verdicts clustered at the PASS/WARN boundary (0.78–0.87 confidence), then shipped anyway. EGPS (v10.0.0) replaced prose verdicts with binary exit-code verdicts from executable predicate scripts — solving the gaming problem at the signal-source level.

The natural development discipline that emerged across cycles 40–60 was **RED-first predicate authorship**: write the predicate script before the production code, verify it exits non-zero (RED) against the current codebase, then implement the production code until the predicate exits 0 (GREEN).

This ADR formalizes that discipline.

---

## Decision

Formalize RED-first predicate authorship as the standard EGPS development discipline:

1. **RED-first**: Write the predicate before the production code. Verify exit_code ≠ 0 on the current codebase. Document the RED→GREEN transition in the build report.
2. **Mutation kill-rate ≥ 0.8**: `mutate-eval.sh` validates tautology resistance. `gate_discover_to_build` blocks on kill-rate < 0.8 when `EVOLVE_MUTATION_GATE_STRICT=1`.
3. **Validator clean**: `validate-predicate.sh` must exit 0 (no banned patterns, metadata headers present, executable bit set).
4. **Anti-tautology AC**: Every predicate must include at least one AC that explicitly tests the failure path — an input that should produce a different outcome, proving the predicate is not always-GREEN.
5. **`mktemp -d` fixture pattern**: Predicates use `TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT` for isolated fixtures. No shared state between predicate runs.
6. **PROJECT_ROOT write discipline**: After writing predicates in a worktree, copy them to `$EVOLVE_PROJECT_ROOT/acs/cycle-N/` before running `run-acs-suite.sh`. Predicates visible only in the worktree branch are invisible to the audit phase running from PROJECT_ROOT.

---

## Consequences

**Positive:**
- Predicates accumulate in `acs/regression-suite/cycle-N/` as an ever-expanding safety net — every shipped cycle adds permanent regression coverage.
- Binary exit-code verdicts eliminate confidence-cliff reward hacking structurally (not patchable by auditor prompt tuning).
- Anti-tautology ACs prevent predicates that always pass regardless of implementation.
- Hermetic fixture pattern (`mktemp -d`) ensures predicates are fast, isolated, and reproducible.

**Negative:**
- Per-cycle overhead: 2–5 predicate scripts per acceptance criterion set (~60–100 lines each).
- Worktree visibility gap: predicates written to a worktree branch are invisible to PROJECT_ROOT audit until ship. **Mitigation:** Builder copies `acs/cycle-N/*.sh` to PROJECT_ROOT as an explicit step before self-certifying.
- Predicate authors must understand both the acceptance criteria AND the test seams to write non-trivial ACs.

---

## References

- EGPS design: `docs/architecture/egps-v10.md`
- Predicate validator: `scripts/verification/validate-predicate.sh`
- Suite runner: `scripts/lifecycle/run-acs-suite.sh`
- Mutation tester: `scripts/verification/mutate-eval.sh`
- ADR-1: LLM router (`resolve-llm.sh`) — example of predicate-driven development
- Incidents that motivated EGPS: `docs/incidents/` (cycles 102–111 confidence-cliff)
