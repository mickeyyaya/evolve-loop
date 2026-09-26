# Cycle 1712 Dossier

**Goal:** Pipeline-health verification, wave 15 (2026-09-27, on main abc9ced0, the first wave with ADR-0106 P3, Q1 and Q2 live: every tmux phase pane's resolved-prompt.txt ends with the identity statement ("## Who you are (stated by the evolve bridge)") and its pane exports CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false before the launch; a quota wall met on a correction re-dispatch, a remediation re-run or a resumed review leaves the cycle DEFERRED (all_families_exhausted in the ledger, no failure digest, no retrospective), never FAIL; a login prompt benches the family with operator_action in cli-health.json and no second dispatch goes into it). Work the highest-weight lane-eligible inbox items end-to-end — claim, tdd, build, audit, ship — with full phase integrity, on the fixed plane. That includes everything waves 3–14 verified, and the fixes on this main:
- (P3) an agent that inspects its pane finds its own session, its prompt files and the sole-writer fact stated in the prompt it was given; a stdout-completion phase is told the bridge reads its pane;
- (Q1) a walled correction re-dispatch is the same deferral as a walled first dispatch;
- (Q2) a credential wall is benched until a probe after the operator's login clears it; the bench lines name the fix;
- (P4, H1, H2, from wave 14) an exit-85 escalation carries its pattern in cause_code; the host derives triage-decision.json from the report before any judge.

Earlier fixes still in force: a worktree ship binds its own tree (#648); routing never sends a lane work its builder's sandbox forbids; a lane's cycle-state override applies only inside its own evolve dir; a Build may `git rm` its own never-committed explanation draft (#651); the explanation contract can move base without a Build when the host proves the change byte-identical (#649).

Every cycle must:
- commit to a claimed inbox item (an empty top_n must not run the spine);
- bind its audit to the changes tree it ships;
- disposition the cycle's OWN defect ledger as well as inherited ids;
- record a terminal outcome on every exit;
- never modify a protected control-plane file by any tool;
- leave no stale worktree behind.

Pipeline-integrity items and items whose fix surface is protected are console-owned, not lane work. ADR-0099 document deliverables are eligible. The measure of this wave is consecutive shipped cycles: 1706 shipped, then 1707 failed on form and 1708/1709 deferred on capacity, so the streak stands at zero; six in a row is the goal.
**Final verdict:** PASS
**Run ID:** 01M3FP1E90X470Y73YA8RZVAWD
**Commit:** ae465766f8692577ceaeb497acd7b0a82c2a1002
**Committed:** `lineage-datestamp-normalization`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 1m53s |  |
| triage | plan | PASS | 59s |  |
| fault-localization | plan | PASS | 3m53s |  |
| bug-reproduction | evaluate | PASS | 3m36s |  |
| tdd | plan | PASS | 7m59s |  |
| build | build | PASS | 54s |  |
| audit | evaluate | FAIL | 6m59s |  |
| tdd | plan | PASS | 8m17s |  |
| build | build | PASS | 6m15s |  |
| audit | evaluate | PASS | 4m45s |  |
| ship | control | FAIL | 1m31s |  |
| audit | evaluate | PASS | 4m36s |  |
| ship | control | PASS | 1m29s |  |

## Timing

**Total:** 53m7s across 13 phases (0 retried) · **Longest:** tdd 8m17s

| Archetype | Wall-clock |
|-----------|------------|
| build | 7m9s |
| control | 3m0s |
| evaluate | 19m57s |
| plan | 23m1s |
