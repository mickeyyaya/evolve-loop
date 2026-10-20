# internal/cycleoutcome

> Related: [ADR-0072](../adr/0072-system-failure-policy-and-halt.md) (the task-level retry ceiling and the system-level release), [ADR-0074](../adr/0074-typed-signal-contracts-routing-authority.md) (the routing authority), and [internal-recurrence.md](internal-recurrence.md).

## Purpose

`internal/cycleoutcome` is the importable closeout seam for a cycle that did not ship. `ApplyFailure` walks the committed ids of a failed cycle through the inbox failure lifecycle. It claims an id if the lane never did, bumps `failure_count`, and quarantines at the ceiling. Uncommitted menu ids release untouched. `ApplyNoWork` is the sibling for a lane whose triage answered for its scoped item without committing it. 

`ApplyFailure` also routes the item of a cycle that stopped on a tdd predicate that reads a protected file. `FailureInputsFor`, `IsTaskLevelFailure`, `IsTaskLevelResult`, `CommittedIDsFor` and `LaneScopeIDs` give the callers one definition of the rules that the closeouts share.

## Design

- **A package, not inline `main` code.** The failure closeout was an inline block in the sequential branch of `cmd_loop.go`. `package main` cannot be imported. So no predicate can pin the behavior. The wave path cannot reuse it either: its lanes run `evolve cycle run` as a subprocess, and `fleet.Result` carries neither cycle number nor workspace.
- **The no-work closeout records what it handed off.** `ApplyNoWork` writes `committedset.Handoff` with the ids it routed to the console, so the loop's `no-work-handoffs` rule counts hand-offs, not every no-work outcome. A failed write is an error of the closeout. Pinned by `TestApplyNoWork_RecordsTheHandOffOnlyWhenAnItemWentToTheConsole` and `TestApplyNoWork_AnUnwritableHandOffRecordIsAnError`.
- **The effect of the old shape.** Fleet-dispatched work never bumped `failure_count`. The ADR-0072 retry ceiling was structurally unreachable. One batch burned four FAILs on the same ids with the count stuck at 0. Both paths now call `ApplyFailure`, so the two halves cannot drift apart.
- **The committed set comes from the workspace.** The `Workspace` (`.evolve/runs/cycle-N`) is the sole source of the committed set, through `triage-decision.json`. So the seam bumps exactly the ids that triage worked, never the whole claimed menu of a lane. `CommittedIDsFor` is the one workspace-level reader. The failure seam and the loop try accounting cannot disagree about the worked set.
- **A missing decision is not a no-op.** An absent, unreadable or malformed decision yields a nil set. That is the legacy whole-directory drain. A missing decision must never become a silent no-op that strands claims in `processing/`.
- **`LaneScopeIDs` is the triage-less committed set.** Both outcome halves share it: the FAIL closeout here and the PASS promotion in `phases/ship/postship.go`. One reader means the two sides cannot disagree about what a triage-less cycle worked. It delegates to the projection that the TDD and Build contracts use.
- **What counts as the fault of the task is one definition.** `IsTaskLevelFailure` blames the task only for classes that may accrue `failure_count`. Transient infra, quota storms and kernel breaches are the fault of the pipeline, not of the todo. The loop's quarantine surface asks the same question, so the function is exported.
- **The blame rule reads the classes of the retry ladder.** `IsTaskLevelFailure` normalizes the class with `failurelog.NormalizeLegacy` and asks the retry-ladder classifier (`internal/retryladder`). A `capability` or `spec` class blames the task, and `ship-gate-config` stays task-level. An `infra`, `quota` or `unknown` class does not (`TestIsTaskLevelFailure_ReadsTheSentinelVocabulary`). The normalization is necessary because `cycleclassify` returns the class of a phase FAIL sentinel in the `failurelog` spelling (`code-build-fail`), not the legacy spelling (`build-fail`).
- **The charge carries the failure record.** `FailureInputsFor` also derives `FailureClass` (`failureClassOf`: the ladder class, and `quota` for the empty-output marker) and `Fingerprint` (`retryladder.Fingerprint` of the first reason in `audit-fail-reason.json`). The drain writes both on each charged item as `last_failure_class` and `last_failure_fingerprint` ([retry-ladder.md §5](../retry-ladder.md#5-where-the-ladder-runs)).
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

## Protected predicate route (tdd)

> Request: the console inbox item `protected-predicate-routes-not-fails`. "A cycle whose own ACS predicate needs a protected-surface edit seals FAIL; it must end as planned no-work and route the item to the console."

### Terms used throughout

- **Protected need**: a path that `guards.IsProtectedSurface` accepts and that a tdd predicate reads.
- **Route**: the console-manual stamp of an inbox item (`route`, `routed_reason`, `routed_cycle`, `routed_at`).
- **Route code**: the diagnostic code `PROTECTED_PREDICATE_ROUTE`. The termination reason is `tdd-protected-predicate-route`.

### Problem

Cycle 1933 claimed `audit-eval-existence-path-convention`. The declared files were `go/internal/phases/audit/` and `go/acs/`.

**F1.1** The tdd phase wrote predicate `TestC1933_004_auditor_eval_existence_rule_names_path_and_rejects_acs`. It reads `agents/evolve-auditor.md` (`go/acs/cycle1933/predicates_test.go:57` in the worktree of the cycle).
*Implication:* the declared files of the item did not show the need. The need appeared only when tdd wrote the predicate.

**F1.2** `agents/evolve-auditor.md` is a protected surface (`go/internal/guards/integrity_surface.go:142`). The build floor restores any edit to it (`go/internal/core/build_floor_reviewer.go:47`).
*Implication:* the builder can satisfy the predicate or the floor, but not both. Cycle 1933 spent five build panes and two corrections, then sealed FAIL.

**F1.3** The routing rule acts on two inputs only: the declared files at claim time (`inboxbatch.ConsoleRouted`) and the `files` of a triage card (`go/internal/phases/triage/protected_route.go:19`). Neither input sees a predicate.

**F1.4** A phase refusal already has a route. `RefusalDisposition` (`go/internal/cyclestate/result.go:94`) names the codes that route the item. `ApplyFailure` routes the subject in place, then drains the claims with no failure bump.

**F1.5** The counters read the verdict. `cycleRunExitCode` returns 2 for `FAIL` (`go/cmd/evolve/cmd_loop_systemfailure_halt.go:14`). The ship streak counts a cycle as shipped, or not (`go/internal/dashboard/history.go:156`). A `SKIPPED_UNKNOWN` cycle exits 0. No counter sees a route.

### Approaches

| Id | Approach | Input that it trusts | Outcome of the cycle | Verdict |
|---|---|---|---|---|
| A | Check the predicates when tdd ends. Stop the cycle, route the item, record a FAIL. | The predicate source, parsed by the host. | FAIL with the route code | Chosen |
| B | Classify the build deadlock as a route. | `build-report.md` prose, which the builder writes. | Any | Rejected: a builder can relabel any FAIL as a deadlock. |
| C | Scan the likely surface of the item at claim time. | The title, summary and declared files of the item. | Any | Rejected: cycle 1933 declared two clean directories, so a guess misses it. |
| D | Extend the triage card check to read predicates. | The card line. | Any | Rejected: triage runs before tdd, so no predicate exists yet. |
| E | Approach A, but record planned no-work (a neutral outcome). | The predicate source. | `SKIPPED_UNKNOWN`, exit 0 | Rejected: the adversarial review found the escape hatch. |

**D1** Choose approach A. It is the first point where the predicate exists. The host parses that source and proves the path. It stops before the build pays for the deadlock.

**D2** Record the cycle as a FAIL. Route the item, and keep the cycle outcome apart from the route.

Approach E failed the adversarial review. A tdd agent can add one `filepath.Join(root, "agents", "evolve-auditor.md")` read, even in dead code. An expected FAIL then becomes a neutral cycle. Repeated neutral cycles never trip the fail breaker or the zero-ship halt. With D2, the gamed route gains nothing: the cycle still counts as a FAIL. The item still stops returning to deadlock lanes.

Approach E also reversed acceptance line 3 of the inbox item ("the routed cycle does not count as a FAIL"). This design reverses that line on purpose, for the reason above.

### The rule

One rule, in one place: the `PhaseTDD` branch of `selectNext` (`go/internal/core/cyclerun_select.go:48`), next to the triage termination. Core keeps only the decision. The scan is in `internal/acspathscan`. The artifact schema is in `internal/committedset`.

1. The host scans `go/acs/cycle<N>/` of the active worktree (`acspathscan.Needs`).
2. A path counts only inside a `func Test*` body. It must be an argument of a file read: `os.ReadFile`, `os.Stat`, `os.Lstat`, `os.Open`, `os.OpenFile` or an `acsassert` file helper. A variable counts if it carries the path to such a call.
3. A path is a whole-path literal, or a `filepath.Join` of literals. A `Join` can start with the repo root (`acsassert.RepoRoot(t)`, or a variable that holds it). A temp directory base does not count.
4. Unused variables, `_ =` lines, message calls, struct tags, prose and helpers that are not tests do not count.
5. Each path must pass the membership function that the composition root injects (`protectedMembership` in `go/cmd/evolve/cmd_cycle.go`). Without it, nothing routes.
6. The committed set must hold exactly one item.
7. The host copies the predicate files into the run directory (`tdd-protected-predicates/`). It writes `tdd-protected-route.json` (cycle and item). It records the tdd ending as `FAIL` with the route code, and the termination `tdd-protected-predicate-route`. The cycle then takes the normal FAIL path. The worktree is kept, and the ledger and dossier stay consistent.
8. A scan fault (a bad pattern, a file that does not parse) writes a WARN. The paths found in the other files still count.

The closeout is the existing failure closeout. `RefusalDisposition` maps the route code to the task level and the console route. `ApplyFailure` then does these steps.

1. It reads the artifact. It requires the same cycle and the same one committed item.
2. It scans the kept copy of the predicates again, with the same membership function. The artifact holds no path. A path in the run directory is never trusted.
3. It adds each proven path to the `files` of the item. A path that is already there is not added twice.
4. It calls `RouteConsole`. The reason names each file, the test function and the predicate file.
5. It drains the claims with no failure bump.

If any step fails, the closeout writes a `[cycleoutcome] WARN` line. The item takes the normal task-level path: bump, release the claim and no route. No claim stays in `processing/`.

### Worked example

Take cycle 1933 with the rule. The predicate joins `RepoRoot`, `agents` and `evolve-auditor.md`, and passes the path to `acsassert`. The path passes the manifest. The only committed item is `audit-eval-existence-path-convention`.

The cycle stops after tdd with the verdict `FAIL` and the code `PROTECTED_PREDICATE_ROUTE`. The item gets `route: console-manual`, `routed_cycle: 1933` and `files` that gain `agents/evolve-auditor.md`. `wave status` shows the verdict FAIL and the line "Termination: tdd-protected-predicate-route". `cyclehealth` shows `FAILED_EXPLAINED` with the same reason.

### Why a lane cannot relabel a failure

- **The cycle still counts.** The verdict is FAIL, the exit code is 2, and the ship streak and the fail breaker see the cycle. Gaming the route gains nothing.
- **The proof is a path, not a claim.** The path comes from predicate source that the host scans twice. The build report is never read.
- **A denied path stays a normal FAIL.** The closeout bumps the item and says why.
- **The route is not a drop.** The item stays in the inbox with an operator stamp. The console reads the file, the test function and the kept predicate, and decides.
- **Limit.** A path that a variable or a helper builds is not found. The cycle then follows the normal path and seals FAIL, as before.
- **Limit.** A tdd phase can still write a read that the task does not need. The cost is one console review of an item, and the cycle counts as a FAIL.

### What changes our mind

Count `PROTECTED_PREDICATE_ROUTE` cycles per wave. If most routes prove to be false, tighten rule 2. If a measured miss rate shows hidden paths, add the helper case. Each added input must stay a path that the manifest proves.

### Tests

- `acspathscan`: `TestNeeds_*` and `TestSnapshot_*` (100% coverage, strict).
- `core`: `TestRunCycle_APredicateThatReadsAProtectedPathStopsAfterTddAsAFailureWithTheRouteCode`, `TestRunCycle_APredicateThatDoesNotReadAProvenProtectedPathRunsTheSpine`, `TestRunCycle_NoProofMeansNoRoute`, `TestRunCycle_ARouteThatCannotBeRecordedLeavesTheCycleOnItsNormalPath`, `TestRunCycle_AScanFaultIsLoudButTheFindingsStillCount`.
- `cycleoutcome`: `TestApplyFailure_AProvenProtectedPredicateRoutesTheItemWithItsFileAndReason` and `TestApplyFailure_AnUnprovenRouteReleasesTheClaimBumpsAndSaysSo`.
- `cmd/evolve`: `TestCloseoutCycleOutcome_AProvenProtectedPredicateRoutesTheItemButStaysAFailure`, which also pins exit code 2.
- `wave` and `cyclehealth`: the termination reason is shown.

## Invariants

- **A closeout never invents an id.** It touches only ids of the committed set of the cycle, or of the scope of the lane for no-work.
- **A routed item keeps its first evidence.** `itemRouteState` finds an inbox item in the root or in any claim directory. It reports if the item already carries a console route stamp. An unparseable record counts as not stamped. An item that is located but cannot be read counts as present and not routed.

## Findings

- From 2026-10-13 to 2026-10-20, no lane FAIL charged its item. Each FAIL sentinel classed as `code-build-fail` or `code-audit-fail`, and the blame rule knew only `build-fail` and `audit-fail`. So the drain ran as system-level. The item `history-refusal-names-the-evolve-comments-command-lane` failed in cycles 1958 and 1966 and kept `failure_count` 0. The `audit-fail-reason.json` files of the two cycles and the release lines of the ledger show this. `TestSealedLaneFail_RaisesFailureCountAndRecordsTheLadderClass` replays that shape.
- A refusal route that failed to fire left the poisoned card to loop. The per-item breaker and the retry ceiling now back each other up ([incident](../../incidents/2026-09-14-triage-refusal-poison-loop.md)).
