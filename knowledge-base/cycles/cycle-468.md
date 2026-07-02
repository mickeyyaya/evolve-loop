# Cycle 468 Dossier

**Goal:** GOAL: TWO SEQUENCED TASKS — the last pre-release hardening. OPERATOR PRIORITY OVERRIDE: outranks all carryover/inbox; triage commits ONE task per cycle in this order.

T1 (cycle 1) — EGPS FLAKE RETRY-ONCE (the contention flake has burned 4 verified-good cycles: 444/447/466/467): in the EGPS/acs verdict generation path (go/internal/phases/audit/audit.go generateACSVerdict + go/internal/acssuite), a RED predicate whose failure is a test failure (not a compile error) gets ONE retry of just the red predicates' scope; passes-on-retry => record GREEN with a "flaky:passed-on-retry" annotation in acs-verdict.json (visible, never silent) + a WARN naming the test; red-on-retry => stays RED (real regression). CONTEXT: flakes spiked after parallel_evaluate=enforce because the -race full-suite predicates now run under concurrent evaluate-phase host load — contention-sensitive tests (tmux/timing) flap. Do NOT weaken the gate: retry is bounded to once, only for test-failure reds, annotated. Red-first test: a deliberately-flaky fixture predicate (fails first run, passes second via a state file) must yield GREEN+annotation; a deterministic-red fixture stays RED. Golden: cycle-466/467 evidence shapes as fixtures.

T2 (cycle 2) — ADVISOR LIVENESS GOLDEN (hardening; tier emission ALREADY LIVE since cycle 467 — deep/balanced routing observed, P3 overlay logs firing): (a) add the per-phase cli+tier fields to the composed plan prompt's response EXAMPLE (buildPlanPrompt/composePlanPrompt, go/internal/core/phase_advisor.go) so emission survives persona drift — currently only the persona elicits it (single point of failure; the example at advisor-prompt-plan.txt line ~31 still shows mint-only); (b) THE LIVENESS GOLDEN: a test composing the REAL production plan prompt asserting the per-phase example contains tier (the artifact the LLM mimics — the test class that would have caught both prior dormancy layers); (c) verify-and-pin the P3 log line shape with a golden.

CONSTRAINTS: strict TDD red-first; go test -race green; apicover -enforce clean; no gate weakening beyond the bounded annotated retry; degrade paths byte-identical.
**Final verdict:** FAIL
**Run ID:** 01KWJ4PVZWYRT1WRX25K0W0E72

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| cycle-recorded |  | FAIL |  | cycle completed; ledger walk deferred to future slice |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 468

