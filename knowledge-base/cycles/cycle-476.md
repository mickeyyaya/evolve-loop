# Cycle 476 Dossier

**Goal:** GOAL: T2 — ADVISOR TIER-EMISSION DETERMINISM (prompt-example hardening). OPERATOR PRIORITY OVERRIDE: outranks all carryover/inbox; triage commits THIS as top_n item 1.

CONTEXT (verified): advisor tier emission is LIVE but INTERMITTENT — cycle 467 tagged 10 phases with {tier}, cycle 475 tagged 5, cycle 474 tagged ZERO. The advisor DOES differentiate correctly when it emits (deep=judgment: tdd/build/audit/adversarial; balanced=scan/triage/review; zero fast — respects the low-model floors). Root cause of intermittency: the {cli,tier} fields are described in the advisor PERSONA prose but are NOT present in the per-phase response-schema EXAMPLE inside the composed plan prompt (buildPlanPrompt/composePlanPrompt, go/internal/core/phase_advisor.go). The ONLY tier in advisor-prompt-plan.txt is the MINT example (line ~31). LLMs reliably mimic the EXAMPLE, not the prose => ~50-100% emission instead of 100%. The P3 overlay log IS working ('no advisor overlay (profile default)' / 'advisor overlay cli=.. tier=..' lines confirmed live in the 2-lane run).

FIX (strict TDD red-first):
(a) PROMPT EXAMPLE: add optional cli+tier to a run=true entry in the per-phase response EXAMPLE the plan prompt shows, with the tier policy inline (deep=judgment-heavy phases that write source/make verdicts; balanced=review/scan/triage; fast=simplest mechanical whitelist ONLY, never source/verdict/scoping; higher-when-in-doubt — operator low-model rule). Keep fields optional (absent => profile default, degrade byte-identical).
(b) LIVENESS GOLDEN: a test that composes the REAL production plan prompt (the actual buildPlanPrompt/composePlanPrompt output) and asserts the per-phase entry EXAMPLE contains the tier field — the artifact the LLM mimics. This is the test class that would have caught both prior dormancy layers (schema-unwired, then example-missing).
(c) pin the P3 overlay-log shape with a golden (it works; lock it).

CONSTRAINTS: strict TDD red-first; go test -race green; apicover -enforce clean; no clamp/floor relaxation; degrade byte-identical when fields absent; reuse existing seams.
ACCEPTANCE: the composed production plan prompt's per-phase example contains {cli,tier}; a golden pins it; (runtime) the next cycles' advisor-response-plan.txt emit tiers for run=true phases consistently.
**Final verdict:** FAIL
**Run ID:** 01KWJTRC91MBVHGSX93RG2FEMJ

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| cycle-recorded |  | FAIL |  | cycle completed; ledger walk deferred to future slice |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 476

