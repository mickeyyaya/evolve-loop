# internal/cycleoutcome

> Related: [ADR-0072](../adr/0072-system-failure-policy-and-halt.md) (the task-level retry ceiling and the system-level release), [ADR-0074](../adr/0074-typed-signal-contracts-routing-authority.md) (the routing authority), and [internal-recurrence.md](internal-recurrence.md).

## Purpose

`internal/cycleoutcome` is the importable closeout seam for a cycle that did not ship. `ApplyFailure` walks the committed ids of a failed cycle through the inbox failure lifecycle. It claims an id if the lane never did, bumps `failure_count`, and quarantines at the ceiling. Uncommitted menu ids release untouched. `ApplyNoWork` is the sibling for a lane whose triage answered for its scoped item without committing it. `FailureInputsFor`, `IsTaskLevelFailure`, `IsTaskLevelResult`, `CommittedIDsFor` and `LaneScopeIDs` give the callers one definition of the rules that the closeouts share.

## Design

- **A package, not inline `main` code.** The failure closeout was an inline block in the sequential branch of `cmd_loop.go`. `package main` cannot be imported. So no predicate can pin the behavior. The wave path cannot reuse it either: its lanes run `evolve cycle run` as a subprocess, and `fleet.Result` carries neither cycle number nor workspace.
- **The effect of the old shape.** Fleet-dispatched work never bumped `failure_count`. The ADR-0072 retry ceiling was structurally unreachable. One batch burned four FAILs on the same ids with the count stuck at 0. Both paths now call `ApplyFailure`, so the two halves cannot drift apart.
- **The committed set comes from the workspace.** The `Workspace` (`.evolve/runs/cycle-N`) is the sole source of the committed set, through `triage-decision.json`. So the seam bumps exactly the ids that triage worked, never the whole claimed menu of a lane. `CommittedIDsFor` is the one workspace-level reader. The failure seam and the loop try accounting cannot disagree about the worked set.
- **A missing decision is not a no-op.** An absent, unreadable or malformed decision yields a nil set. That is the legacy whole-directory drain. A missing decision must never become a silent no-op that strands claims in `processing/`.
- **`LaneScopeIDs` is the triage-less committed set.** Both outcome halves share it: the FAIL closeout here and the PASS promotion in `phases/ship/postship.go`. One reader means the two sides cannot disagree about what a triage-less cycle worked. It delegates to the projection that the TDD and Build contracts use.
- **What counts as the fault of the task is one definition.** `IsTaskLevelFailure` blames the task only for classes that may accrue `failure_count`. Transient infra, quota storms and kernel breaches are the fault of the pipeline, not of the todo. The loop's quarantine surface asks the same question, so the function is exported.
- **A phase refusal follows its code.** `IsTaskLevelResult` extends the rule to a phase refusal. The `cyclestate.RefusalDisposition` of the code decides. An I/O fault stamped `TRIAGE_COMMITMENT_INVALID` never charges the queue. A card that named a protected surface does. A caller that holds only the class keeps `IsTaskLevelFailure`, where a refusal is not task-level by default.
- **`FailureInputsFor` derives the shared inputs.** It derives the ceiling (from `.evolve/policy.json`, with the compiled default as fallback) and the task-level flag. Both call sites use it, so the classification rule has one definition.
- **The per-item breaker for a protected-surface refusal.** That refusal is deterministic and the operator owns it. So the refused card is routed console-manual in place, wherever the claim of the lane left it. This happens before the drain releases it. The claim floor then refuses every later lane.
- **The route is the disposition of that item.** The other committed ids of the cycle were never worked, because triage stopped the cycle. So the drain runs with `Routed` set: no bump and no park. `SystemLevel` stays the fact that it is. A route that cannot happen says so on the stream. Then the task-level bump runs, and the retry ceiling is the second breaker.
- **An agent-authored id never widens a closeout (ADR-0073).** A `Subject` outside the committed set of the cycle is a mis-copy, not a disposition. The task-level bump runs.
- **Planned no-work hands the item to the console.** A fleet lane can end as planned no-work when triage answered for its scoped item without committing it (a reasoned drop, a skip, an escalation). The item is still pending in the inbox root, because triage builds its menu there. So the next wave draws it into another lane and another no-work end. `ApplyNoWork` routes it to the console in place with the reason of the lane. The claim floor refuses every later lane.
- **The console retires, the lane does not.** The console confirms and retires the item. An unshipped lane never retires work (the anti-laundering principle behind the closure-claim gate). Scoped ids that no inbox item backs are retired as `rejected/cycle-N/` records. Each carries the answer of the lane, or the statement that it answered for nothing.
- **No-work then releases the claims, with no failure bump.** This matches the routed drain of `ApplyFailure`. Triage claims before it selects, and a no-work lane worked nothing. So a routed item arrives console-visible, not stranded in the claim directory that git ignores.
- **Scope edges of no-work.** A cycle without a lane pin has no scope to hand over and releases nothing. A sequential no-work end keeps its historical lifecycle. An answered id outside the scope is never touched. An item that already routed to the console keeps its original evidence. An item that left the inbox is gone, not a route failure.
- **Faults never change the exit code.** Route and release faults are on the stream. They return joined, for the caller to WARN. A lifecycle hiccup never changes the exit code of a cycle.
- **The seams belong to the root.** `Ledger` is the chained-append seam for the lifecycle lines, the observed ledger of the root. If it is nil, the inbox mover falls back to its own unobserved file ledger over the same file. `Signals` is the Signal Center of the root for the `inbox.warning` events of the mover. If it is nil, the mover prints the legacy `[inbox-mover]` line. `WithLedger` and `WithSignals` return a copy and leave the receiver untouched.

## Invariants

- **A closeout never invents an id.** It touches only ids of the committed set of the cycle, or of the scope of the lane for no-work.
- **A routed item keeps its first evidence.** `itemRouteState` finds an inbox item in the root or in any claim directory. It reports if the item already carries a console route stamp. An unparseable record counts as not stamped. An item that is located but cannot be read counts as present and not routed.

## Findings

- A refusal route that failed to fire left the poisoned card to loop. The per-item breaker and the retry ceiling now back each other up ([incident](../../incidents/2026-09-14-triage-refusal-poison-loop.md)).
