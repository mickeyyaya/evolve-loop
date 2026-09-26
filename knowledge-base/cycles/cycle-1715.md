# Cycle 1715 Dossier

**Goal:** Pipeline-health verification, wave 16 (2026-09-27, on main cb36eae6, the first wave with ADR-0106 W1, H1b and X1 live beside P3, Q1 and Q2: a shipped or declined id no inbox item backs leaves a retirement record (processed/ or rejected/cycle-N) and is never re-pinned; an empty optional bucket in triage-report.md is no cards; .evolve/inbox/ records are non-material to the explanation document, so no floor asks a builder to describe the ship's consumption stamp; and every tmux phase pane's resolved-prompt.txt ends with the identity statement ("## Who you are (stated by the evolve bridge)") and its pane exports CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false before the launch; a quota wall met on a correction re-dispatch, a remediation re-run or a resumed review leaves the cycle DEFERRED (all_families_exhausted in the ledger, no failure digest, no retrospective), never FAIL; a login prompt benches the family with operator_action in cli-health.json and no second dispatch goes into it). Work the highest-weight lane-eligible inbox items end-to-end — claim, tdd, build, audit, ship — with full phase integrity, on the fixed plane. That includes everything waves 3–14 verified, and the fixes on this main:
- (P3) an agent that inspects its pane finds its own session, its prompt files and the sole-writer fact stated in the prompt it was given; a stdout-completion phase is told the bridge reads its pane;
- (Q1) a walled correction re-dispatch is the same deferral as a walled first dispatch;
- (Q2) a credential wall is benched until a probe after the operator's login clears it; the bench lines name the fix;
- (P4, H1, H2, from wave 14) an exit-85 escalation carries its pattern in cause_code; the host derives triage-decision.json from the report before any judge;
- (W1, H1b, X1, from wave 15) a lane pinned to an id no inbox item backs ends once, with a record; a `## superseded` with nothing under it derives to []; the explanation floor never names an inbox record as a material path.

Earlier fixes still in force: a worktree ship binds its own tree (#648); routing never sends a lane work its builder's sandbox forbids; a lane's cycle-state override applies only inside its own evolve dir; a Build may `git rm` its own never-committed explanation draft (#651); the explanation contract can move base without a Build when the host proves the change byte-identical (#649).

Every cycle must:
- commit to a claimed inbox item (an empty top_n must not run the spine);
- bind its audit to the changes tree it ships;
- disposition the cycle's OWN defect ledger as well as inherited ids;
- record a terminal outcome on every exit;
- never modify a protected control-plane file by any tool;
- leave no stale worktree behind.

Pipeline-integrity items and items whose fix surface is protected are console-owned, not lane work. ADR-0099 document deliverables are eligible. The measure of this wave is consecutive shipped cycles: 1712 shipped (its code verified on the first audit; one explanation sentence about a host-stamped inbox record cost it a round), 1713 ended planned no-work on a pin already shipped; the streak stands at one; six in a row is the goal.
**Final verdict:** PASS
**Run ID:** 01M3FVE82JAE5G9M7MAJCV7EGH
**Commit:** 85af083f743525e51448aa053b4acfff5e7fd1e9
**Committed:** `acs-cycle1529-doc-only-predicate-diffs-against-local-main`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 1m44s |  |
| triage | plan | PASS | 58s |  |
| bug-reproduction | evaluate | PASS | 4m35s |  |
| tdd | plan | PASS | 6m54s |  |
| build | build | PASS | 4m18s |  |
| audit | evaluate | PASS | 4m56s |  |
| ship | control | FAIL | 10s |  |
| audit | evaluate | PASS | 3m43s |  |
| ship | control | PASS | 11s |  |

## Timing

**Total:** 27m28s across 9 phases (0 retried) · **Longest:** tdd 6m54s

| Archetype | Wall-clock |
|-----------|------------|
| build | 4m18s |
| control | 21s |
| evaluate | 13m13s |
| plan | 9m36s |
