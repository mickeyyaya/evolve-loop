# Triage Autonomous Goal Divergence — Cycle 96 (2026-05-20)

**Status:** Working as designed (documented as operator expectation)
**Severity:** LOW (no integrity issue; operator transparency issue only)
**Functional impact:** Cycle 96 shipped different work than operator-stated goal text. The work shipped is valid (matches carryover backlog priority); the operator's stated work was deferred to cycle 97.
**Structural impact:** Confirms that the triage system's autonomous priority calculation outweighs operator goal text when carryover backlog has higher-priority items. This is intentional system design, not a defect.

## 1. What happened

Cycle 96's dispatcher was invoked with goal text "P4 triage Layer-3 extraction + L1 orchestrator context_mode digest" (from the token-economics roadmap plan). The triage agent ran scout's discovery output, weighed it against `state.json:carryoverTodos[]`, and chose to ship a different scope: "builder turn-18 STOP CRITERION + mastery consecutiveSuccesses increment" (cycle 96 commit `1f40061`).

The operator-stated P4+L1 were deferred to cycle 97, which then shipped L1 (commit `a10ca24`) and found P4 was already done in a prior cycle (`agents/evolve-triage-reference.md` existed). Net effect: P4 was correctly identified as already-shipped, and the plan's intended work was completed across cycles 96-97 with the triage system's autonomous re-prioritization.

The operator's expectation gap: assumed `--cycles N` + goal text = exact execution of goal text. Actual behavior: triage treats goal text as one input among several (scout findings, carryoverTodos priority, mastery state, project history).

## 2. Research

### Triage decision sources (in priority order, observed)

1. **state.json:carryoverTodos[]** — explicit operator-queued items (HIGH priority by default; deferred items decay via `cycles_unpicked` counter per `scripts/lifecycle/reconcile-carryover-todos.sh`)
2. **state.json:failedApproaches[]** — adapter signals from prior cycles (forces avoidance of repeated failure modes)
3. **scout-report.md backlog** — discovery findings from the current cycle's scout
4. **state.json:abnormalEvents[]** — recent warning events that should be addressed
5. **operator goal text** — the `<goal>` arg passed to `/evolve-loop`

The operator goal text is INPUT #5, not #1. When carryoverTodos contains "builder turn-overrun fix" (carried over from cycle 89's abnormal-events), and scout-report independently surfaces "mastery field plumbing needed" (downstream of cycle-95 P2), the triage agent's `cycle_size_estimate` calculation may rank these higher than the operator's stated work.

### Why this is correct system behavior

The triage system was designed to be autonomous — the project's whole thesis is "self-evolving development pipeline" where the system has agency over its own priorities. The operator's goal text is a hint, not a command. If the operator wanted strict goal adherence, they would use a single-shot tool like `claude code "fix X"` rather than the cycle pipeline.

In cycle-96 specifically, the triage decision was VALIDATED in retrospect: P4 turned out to be already-done, so the operator's plan was over-scoped. The triage system's choice to defer P4+L1 actually saved a wasted cycle. Cycle 97 then completed L1 and correctly identified P4 as a no-op.

### Operator expectation calibration

Three modes of operator interaction:
1. **"Suggest" mode (default):** Operator goal text is input #5. Triage may or may not pick it up. Best for self-evolving long-term improvement.
2. **"Insist" mode (not implemented):** Operator wants strict goal adherence. Would require `EVOLVE_GOAL_STRICT=1` env var that forces triage to weight goal text as #1. Not currently available.
3. **"Manual" mode (existing):** Operator skips `/evolve-loop` and does the work themselves via standard Claude Code. Bypasses the cycle pipeline entirely.

For the v10.17.0 batch, the operator was in mode 1. The autonomous behavior was correct but unexpected.

## 3. Reasoning

The cycle pipeline's autonomy is a feature, not a bug. But it creates an operator transparency gap: the operator's stated goal may not match the cycle's actual deliverable. Without documentation, this looks like a defect.

Mitigation paths:
- **Documentation (this dossier):** Operator now knows goal text is an input, not a directive
- **Triage output transparency:** Triage already writes `triage-decision.md` explaining its priority calculation; operator can read this to understand cycle direction
- **Optional --strict-goal flag:** If implemented, would force goal text to #1 priority. Not a current need; documented here for completeness

The decision: leave the autonomous behavior unchanged, document the expectation. The pipeline IS self-evolving by design.

## 4. Fix

No code fix needed. Documentation fix: this dossier + README.md "How Evolve Loop Compares" section should clarify that operator goal text is suggestion, not command.

If operators want strict adherence, the workaround is to phrase the goal text minimally with carryover-style framing:
```
/evolve-loop --cycles 1 balanced "ONLY work on P4 triage Layer-3 extraction. Do not pick up carryover items."
```
This explicit instruction tends to bias triage toward the stated work, though it's not enforced.

## 5. Lessons

- **No new lesson YAML needed** — this is a working-as-designed pattern, not a learning. The system documentation captures it.
- **Cross-reference for operators:** `agents/evolve-triage.md` documents the priority sources; this dossier documents the observed behavior on a real cycle.

## 6. References

- Cycle 96 feat-commit: `1f40061 fix(cycle-96): builder turn-18 STOP CRITERION + mastery consecutiveSuccesses increment`
- Original operator goal: "P4 triage Layer-3 extraction + L1 orchestrator context_mode digest"
- Cycle 97 feat-commit (where stated work was completed): `a10ca24 feat(cycle-97): L1 orchestrator digest-by-default via profile context_mode`
- Triage decision artifact: `.evolve/runs/cycle-96/triage-decision.md`
- Reconciliation script: `scripts/lifecycle/reconcile-carryover-todos.sh`
- Persona: `agents/evolve-triage.md` (documents priority sources)
- Cross-references:
  - [`v10-17-0-release-debrief.md`](v10-17-0-release-debrief.md) — discusses plan-to-actual mapping
