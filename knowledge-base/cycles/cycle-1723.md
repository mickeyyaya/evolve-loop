# Cycle 1723 Dossier

**Goal:** Pipeline-health verification, wave 21 (2026-09-27, on main c521ec1d, the first wave with ADR-0106 W3 live (one dispatchability rule: the plan, the prunes, the refill, the launch gate, the triage menu, `evolve inbox batches` and the sequential no-work check all ask inboxmover's one rule from one root, so the plan never offers a lane the gate refuses and a wave that launches nothing is not a wave) and after the 2026-09-27 inbox curation (25 fixed or obsolete items retired with evidence, 7 duplicates merged, every live item re-weighted by stability, debuggability and performance), and the third wave with the carry live (ADR-0105 B3/B4 = ADR-0106 P1: a ship that meets a moved main, rebases byte-identically and rebinds its explanation ships on the verdict it already earned, with the record chained in the ledger and re-proven by ship, instead of a second audit) beside F7, B5, X3, Q3, R1, R2, W1, H1b, X1, P3, Q1 and Q2: a test-only Build that explains itself anyway is verified, not refused (only foreign cycle records are immutable, in every branch); a capacity wall in the sequential loop pauses the loop before it records a failed approach or runs the failure closeout; a top_n card that names a protected surface is moved by the host into escalate_block and the item routed to the console by the planned no-work closeout, never the cycle's FAIL, and the triage prompt lists the protected surfaces; a shipped or declined id no inbox item backs leaves a retirement record (processed/ or rejected/cycle-N) and is never re-pinned; an empty optional bucket in triage-report.md is no cards; .evolve/inbox/ records are non-material to the explanation document, so no floor asks a builder to describe the ship's consumption stamp; and every tmux phase pane's resolved-prompt.txt ends with the identity statement ("## Who you are (stated by the evolve bridge)") and its pane exports CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false before the launch; a quota wall met on a correction re-dispatch, a remediation re-run or a resumed review leaves the cycle DEFERRED (all_families_exhausted in the ledger, no failure digest, no retrospective), never FAIL; a login prompt benches the family with operator_action in cli-health.json and no second dispatch goes into it). Work the highest-weight lane-eligible inbox items end-to-end — claim, tdd, build, audit, ship — with full phase integrity, on the fixed plane. That includes everything waves 3–14 verified, and the fixes on this main:
- (P3) an agent that inspects its pane finds its own session, its prompt files and the sole-writer fact stated in the prompt it was given; a stdout-completion phase is told the bridge reads its pane;
- (Q1) a walled correction re-dispatch is the same deferral as a walled first dispatch;
- (Q2) a credential wall is benched until a probe after the operator's login clears it; the bench lines name the fix;
- (P4, H1, H2, from wave 14) an exit-85 escalation carries its pattern in cause_code; the host derives triage-decision.json from the report before any judge;
- (W1, H1b, X1, from wave 15) a lane pinned to an id no inbox item backs ends once, with a record; a `## superseded` with nothing under it derives to []; the explanation floor never names an inbox record as a material path;
- (R1, R2, from wave 16) a card on a protected surface is a route, not a verdict: the cycle ends planned no-work (or continues on its other cards) and the item arrives console-manual with the lane's reason;
- (F7, from wave 18) a named red in the ship's importer backstop re-runs its package by itself before it is the lane's RED; green by itself is flake evidence (`SHIP_BACKSTOP_FLAKE` in the Signal Center, "evidence, not proof" in the log) and the ship proceeds; still red is the lane's RED;
- (B5, from wave 18) a debugger that resolves a fleet-rebase conflict hands the tree back to the rebase: the log says "debugger resolved the fleet-rebase conflict; re-entering the fleet rebase on the resolved tree", then the carry, the rebind or "Build re-authors the explanation" — never a reship or a re-audit on the base the ship diverged from;
- (X3, from wave 18) a Build whose diff is non-material (tests, docs, evals, predicates) and whose report declares its own explanation document REQUIRED passes the floor with an empty material set; one that declares NOT_APPLICABLE beside an undeclared own record passes with the record kept as documentation; only a changed FOREIGN cycle record is refused ("published cycle records are immutable"); `bridge/channel.TestChannel_EndToEnd` no longer depends on a 10 ms window under the ship backstop's load;
- (Q3, from wave 18) a sequential-loop cycle walled on capacity stops with `stop_reason: quota-pause`, exit 5, `recoverable_failures` 0 and no entry in failedApproaches;
- (W3, from wave 20) every planned lane launches: no `freshness gate skipped` line for a lane the plan itself chose, no `wave N: 0/0 lanes ok`; a wave with nothing dispatchable takes the min-width repair and the empty-backlog path (`LOOP_WAVE_EMPTY_PLAN cause=empty_backlog`) and counts toward work-supply starvation; the triage prompt lists items waiting on an unlanded dependency under `dependency_blocked` and never offers them; a sequential cycle whose triage commits nothing over console-owned or waiting work ends planned no-work, never `triage-empty-commitment-claimable-work`, even when an inbox item carries a trimmed title or a duplicate id (only an unreadable inbox file is unread work);
- (P1, from wave 17) a byte-identical rebase after a peer's landing carries its audited verdict to ship: the log says "rebase is byte-identical: the audited verdict carries, shipping without a second audit" and ship's binding log names the carry; a carry the host or ship cannot prove is the old path, a second audit.

Earlier fixes still in force: a worktree ship binds its own tree (#648); routing never sends a lane work its builder's sandbox forbids; a lane's cycle-state override applies only inside its own evolve dir; a Build may `git rm` its own never-committed explanation draft (#651); the explanation contract can move base without a Build when the host proves the change byte-identical (#649).

Every cycle must:
- commit to a claimed inbox item (an empty top_n must not run the spine);
- bind its audit to the changes tree it ships;
- disposition the cycle's OWN defect ledger as well as inherited ids;
- record a terminal outcome on every exit;
- never modify a protected control-plane file by any tool;
- leave no stale worktree behind.

Pipeline-integrity items and items whose fix surface is protected are console-owned, not lane work. ADR-0099 document deliverables are eligible. The measure of this wave is consecutive shipped cycles: the goal is six in a row, counted from this wave's first cycle, with every fix above live.
**Final verdict:** PASS
**Run ID:** 01M3J1MBYGPNRTYARWHR6ZZ15M
**Commit:** cb8132f3a43ff2e6fdc3f994530faf0e79dc50c5
**Committed:** `commentaudit-reports-every-added-comment`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 5m38s |  |
| triage | plan | PASS | 1m3s |  |
| bug-reproduction | evaluate | PASS | 3m3s |  |
| tdd | plan | PASS | 10m47s |  |
| build | build | PASS | 4m25s |  |
| audit | evaluate | PASS | 5m36s |  |
| ship | control | PASS | 1m39s |  |

## Timing

**Total:** 32m10s across 7 phases (0 retried) · **Longest:** tdd 10m47s

| Archetype | Wall-clock |
|-----------|------------|
| build | 4m25s |
| control | 1m39s |
| evaluate | 8m39s |
| plan | 17m27s |
