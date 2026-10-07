# Operating Policy (canonical, environment-independent)

> **This document is the canon.** Operator-session memory, CLI-local notes and
> platform config are advisory mirrors of THIS file. A clean environment (a new
> machine, a new operator, any AI CLI) gets the full policy from the repo alone.
> The rules that a machine can enforce are **compiled Go defaults** (`internal/policy`),
> and `.evolve/policy.json` is the override surface. This document holds the process
> rules that no gate can fully mechanize, each with its evidence. Update it the same way
> as code, through the sanctioned review + ship flow, and cite the incident that caused the change.

## 0. Logic-first delivery (P0, the refactor policy)

A cycle exists to deliver logic: the fix, the feature, the architecture that
survives the next change request. The pipeline judges every phase on that logic.
The form of the deliverables of a phase is the job of the pipeline to repair.
The form is:

- the report shape;
- the required sections;
- the artifact names and paths;
- a binding to a tree that moved;
- order and lock collisions;
- the bookkeeping.

1. **Three kinds of block; only two are final.**
   - A *logic* block names a defect in the behaviour of the change. The defect is a test that fails
     or that is not there, a regression, a security finding or an audit finding on substance. A logic block blocks.
   - A *form or process* failure goes to recovery. It blocks only when the recovery is exhausted.
   - The pipeline never recovers an *integrity* block. An integrity block is the treefence, the
     predicate-authority fence, ADR-0072 incoherence, or a sandbox or recovery-guard violation.
     The cycle aborts and the pipeline files a P0.
2. **Recovery is layered, cheapest first.** The layers come in this order:
   1. a host derivation, before any judge;
   2. deterministic rungs in code (relocate, re-bind, retry);
   3. the recovery agent. It restructures a deliverable around the logic that the
      phase already built. It never edits code or tests;
   4. a re-dispatch of the phase.
3. **Recovery never launders.** The gate that rejected a deliverable judges its
   repair. The deliverable earns the verdict again: the pipeline never carries a verdict forward.
   The pipeline never repairs judgment and control phases (§4). The kernel, not the agent, proves
   that nothing outside the grant changed and that every added line has a source.
   The pipeline records every recovery where the auditor and the dossier see it.
4. **The evidence decides: assign the work back, or recover it.**
   - An agent can make an error in the form or the structure of a deliverable. Then the
     pipeline recovers the deliverable toward the original intention only in one case.
   - That case: the work carries enough kernel-checkable evidence that the request is fulfilled.
     The change is present. The tests written for it are present, unchanged since TDD,
     red on the base and green now. The auditor still judges if the tests cover the acceptance.
   - The pipeline always assigns a deliverable that is not there back. In all other cases, the
     pipeline assigns the work back. The correction names the evidence that is not there, never the format.
   - The delivered code and its explanation must speak for themselves. A recovered
     deliverable states only what the evidence shows.
5. **Bounded and loud.** The recovery rounds come from config. The pipeline records exhausted
   recovery as a pipeline defect to fix (§1). Exhausted recovery still counts toward the
   halts of §1.3, because a pipeline that cannot recover is degraded.

*Evidence: over cycles ~1550–1707, a byte-identical change ran Build and
Audit again because a peer landed first (14 cycles). A derivable secondary sent a
whole phase back (4 cycles, and it still recurs). The TDD agent of cycle 1707 refused
its own task for an hour because it did not understand a process. None of these was a
defect in the change. Design and components:
[ADR-0106](../architecture/adr/0106-logic-first-delivery.md).*

## 1. Pipeline-integrity policy (highest severity)

A **pipeline-integrity defect** is any defect that causes false cycle verdicts,
corrupts shared state, breaks gates/CI on main, or makes failures
undiagnosable. Its blast radius is every later cycle.

1. **Fix directly, never queue-only.** A loop cannot repair its own foundation
   while it stands on it. The defect gets an inbox item as the *record*
   (evidence, acceptance criteria). But a console/operator session does the
   fix immediately, in this sequence:
   1. an isolated worktree branch;
   2. TDD red-first;
   3. dual review (+ security review when the change is trust-kernel-adjacent);
   4. a wiring proof;
   5. the sanctioned commit-gate/ship;
   6. a CI watch.

   *Evidence: cycles 999–1001 burned on the exact state defect that the queue had
   to fix. The console fix landed in hours (PR #345).*
2. **Maximum reasoning, implicitly.** Pipeline issues always get the deepest
   analysis and the highest model tier. This applies to the operator session, to every
   review subagent, and to the loop phases that touch pipeline classes. Never
   use the default tier for a pipeline diagnosis.

   *Evidence: the verdict-incoherence family used up five fixes made from inference,
   and each fix was refuted. One fingerprint-first deep pass found the
   real cause. Deterministic defect strings outrank every narrative.*
3. **The loop halts itself on blockers.** Forged verdicts halt the loop instantly
   (ADR-0072 floor). Other blockers halt the loop at policy ceilings (blocker breaker,
   `failure_policy.thresholds`). These blockers are failure fingerprints that recur,
   guard-abort classes, and runs of failures without a reason. Every halt auto-files a P0
   `pipeline-defect` item. The queue gets the item even while the loop stops.
4. **Salvage before requeue.** After any failed/aborted cycle, inventory the
   preserved worktree before you queue the work again. Land recoverable work through the
   operator flow.

   *Evidence: the seven FAILs of batch-5 gave three complete,
   landable work items (PRs #350/#351/#352), with zero re-implementation.*

## 2. Queue policy

1. **Never stop the queue.** For a task-level failure: classify, learn, continue.
   Only a SYSTEM-level failure halts the loop (ADR-0072). The halt itself files the
   P0 fix item.
2. **Routing is typed plumbing, not prose** (ADR-0074). Inbox items carry
   `route`:
   - `console-*` = operator-owned. The pipeline structurally refuses the item at plan time
     and at claim (exit 3).
   - `"lane"` = an explicit override for a false positive of the derivation.
   - absent = the pipeline derives the route from the protected-surface manifest.

   *Evidence: in batch-5, prose fences were ignored twice on one item. Six of
   seven FAILs were control-plane draws. The gate now makes these draws impossible.*
3. **Weights are priority, routes are authority.** Never use one for the other.
   Pipeline-class items occupy the 0.85–0.97 band.
4. **Tracked-path main-tree writes happen only at batch boundaries.** This rule applies
   to operators too. Only `.evolve/inbox/` is safe while lanes run.

   *Evidence: three innocent cycles (1011/1023/1027) died because of mid-batch console
   writes. One cycle (1043) died because of a mid-wave queue hold.*

## 3. Engineering standards (all work, loop or console)

1. **TDD red-first; every bug fix lands with a regression test.** A test that
   never failed proves nothing.
2. **Dual review before commit** (simplifier + language reviewer through
   commit-gate). An architecture change also gets an adversarial architect review.
   It must land with the ADR or the design doc that explains it.
   - A pure-docs diff still needs both review capabilities, because docs are the primary
     asset of the project. One `code-review-simplify` diff-review pass covers simplify and review.
     A pure-docs diff can skip the architecture review
     ([ADR-0115](../architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md)).
   - A comment removal needs no reviewer. The commit gate accepts a proof instead when
     every changed Go file is proven comment-only and every other file is Markdown under `docs/`.
     The commit then carries `Review-waived: comment-only` ([code comments](../conventions/code-comments.md)).
   - *Mechanized: the docs floor (`internal/docsfloor`, ADR-0077) WARNs at the
     build handoff when an architecture-labeled change touches no `docs/` file.
     WARN, not block: "is there a doc at all" is mechanical, and "is this doc
     adequate" stays with the auditor.*
3. **Wiring proof (the I2 invariant).** A mechanism ships only with proof that its
   output is consumed on the composed live path. Unit-green ≠ live-green.

   *Evidence: retrofile, the recurrence escalator, the claim→quarantine chain
   and the S1 assembler all shipped inert in exactly this way. Only a live-fire
   monitor caught two of them.*
4. **No feature flags; config-injected policy.** Behavior comes from compiled
   defaults + `.evolve/policy.json` (Strategy/DI). It never comes from env-flag sprawl or
   from Go literals for thresholds.
5. **Single source with projection.** Never make a second writer or copy of state,
   vocabulary or logic that can drift. Examples are the statemap writers, the enum vocabularies
   and the routing classifier.
6. **Git discipline.** Do all development on branches. The gate denies a bare
   `git commit`/`push`. Stage explicit paths (never `git add -A` outside
   `/commit`). Ship through the `evolve ship` classes. Release through `evolve release`.
7. **Fail loudly.** Every degraded path WARNs with specifics. "missing
   artifact" errors are writer defects. Treat a silent narrowing as a bug.
8. **Issue/gap/solution docs on every fix (operator directive, 2026-08-04).**
   Every fix (a console PR or a loop cycle) lands with documentation in
   **issue / gap / solution** format:
   - what was wrong;
   - why the current net missed it;
   - how the change closes it.

   Give enough detail that a reader can audit the claim against the diff.
   Extend the relevant review or incident doc
   (for example, [batch-integrity-review-2026-08-04](batch-integrity-review-2026-08-04.md)),
   or link a new doc in the same format.
9. **Ledger writes derive from diffs, not labels.** Queue records, CHANGELOG
   closure claims and memory ledgers must come from the actual diff of the ship
   (`git show --stat`). For "activated/landed" claims, they must also come from runtime
   artifact evidence. They must never come from lane labels or verdict events.
   The audit of a continuation lane must account for the defect list of the ORIGINAL
   audit that rejected the work. It must give a disposition for each defect (FIXED/DEFERRED/OPEN).

   *Evidence: the 2026-08-04 batch integrity review. A reproduced CRITICAL
   was laundered to "verified closed" across four locally-honest hops. A
   dormant feature was recorded as a feature in an active soak.*

## 4. Failure handling (the routed-not-fatal contract)

Every FAIL produces three artifacts:

- a machine-readable reason artifact. The floor writes it, or the retro paths write it as a fallback;
- a deterministic failure digest (fingerprint + pre-class + recurrence);
- a retro `disposition.json` (legitimacy / root-cause layer / salvage / urgency / routing).
  The pipeline cross-checks it against the digest, so that the identity cannot be invented.

Honest rejections stay rejected. The pipeline never buys the pass-rate metric with weaker
judges: a hard deny-list excludes the judgment and control phases from remediation.

### 4.1 Zero-ship halt and the ship-streak goal (canonical home)

This is the one statement of the rule. `CLAUDE.md`, [pipeline-factory-rules.md](pipeline-factory-rules.md)
and [runtime-reference.md](runtime-reference.md) link here and do not restate it.

1. **Two consecutive zero-ship cycles stop the batch.**
   - When you run the evolve loop or any batch/merge-train automation, evaluate the
     outcome after every cycle.
   - If two consecutive cycles produce zero ships (0 merged PRs / landed cycle commits),
     stop the loop. Find the root cause in the pipeline before you run another wave.
   - Never run more than two unproductive waves "to see if it self-corrects".
   - A zero-ship streak is SYSTEM-fail evidence: the ADR-0072 halt and its P0 apply.
     This does not conflict with "never stop the queue", because §2.1 governs task-level failures.
2. **It is an operator guardrail, not a compiled breaker.**
   - No Go code counts zero-ship cycles. The console operator applies the rule.
     The rule came into use after the 2026-08-10
     [absorbing-FAIL incident](../incidents/2026-08-10-continuation-absorbing-fail.md),
     and #430 made it repo policy.
   - The nearest compiled mechanisms are looser and fire later:
     - The `consecutive-failures` rule of the blocker breaker halts after
       `failure_policy.thresholds.consecutive_failures_halt_ceiling` (default 3) back-to-back
       cycles that *fail*, with any fingerprint (`go/internal/core/blocker_breaker.go`, #423).
     - The goal-stall escalation files an item after 3 empty cycles, or 5 cycles that do not ship,
       on one goal (`goal_stall`, `go/cmd/evolve/cmd_loop_goalstall.go`).
   - So an empty streak, or a mixed EMPTY/FAIL streak, reaches the operator rule first.
   - A fleet lane that is DEFERRED because it has no worktree is not a cycle that fails.
     It writes no failure digest. The `lane-deferrals` rule of the breaker halts after
     `failure_policy.thresholds.lane_deferral_halt_ceiling` (default 3) of them in a row.
     So a persistent provisioning fault is a halt, never a silent zero-ship wave after wave
     ([cycle 1806](../incidents/2026-10-06-cycle-1806-fetch-ref-lock.md)).
3. **Ship-streak goal: six consecutive ships.** The pipeline-health target of the operator is six
   consecutive shipped cycles. The target was five consecutive ships during the 2026-09-14
   [verification wave](../research/verification-wave-findings-2026-09-14.md). It moved to six by
   2026-09-26 (the "six-consecutive-ships campaign" of waves 6–8, for example
   [the triage-claim incident](../incidents/2026-09-26-triage-claim-left-to-the-agent.md)).
   The goal measures pipeline health. The halt in item 1 is the stop condition on the way to the goal.

## 5. Release policy

The release trigger is 4 consecutive PASS verdicts in a batch (or the word of the operator).
There is one entry point: `/evo:publish` (`evolve release X.Y.Z`). It does these steps:

- the preflight gates;
- the changelog;
- the atomic version bump;
- the marketplace propagation;
- the post-release CI watch + 15-asset verification;
- an auto-rollback on failure.

"Publish" ≠ "push".

## 6. Model & CLI routing

- Routing uses tiers, not model names (`fast < balanced < deep < top`).
- Any CLI × any phase × any model must execute.
- On quota exhaustion, routing falls back across model families.
- The per-model exhaustion regexes are validated against real provider surfaces.
  They are false-positive-safe: a spurious bench costs a pause, and a missed signal costs a livelock.
- Pipeline-class phases and reviews run deep/top.

## 7. Where the machine-enforced half lives

| Concern | Enforcement | Override |
|---|---|---|
| Gates (eval/contract/EGPS/tdd) | compiled defaults, `internal/policy` | `.evolve/policy.json` `gates`/`workflow` |
| Routing authority | `inboxbatch.ConsoleRouted` + plan-time gate + claim floor | item `route` field |
| Failure thresholds & breaker ceilings | `internal/policy` `failure_policy.thresholds` | `.evolve/policy.json` |
| Build handoff floor / remediation | `workflow.build_floor`, `remediation_rounds` | `.evolve/policy.json` |
| Protected surfaces | `guards.ProtectedSurfaceManifest` (compiled) | operator manual ship only |
| Docs floor (§3.2) | `internal/docsfloor` + build handoff floor (WARN) | `.evolve/policy.json` `docs_floor.stage` |

Related: [runtime-reference.md](runtime-reference.md) ·
[control-flags.md](../architecture/control-flags.md) · ADRs 0064/0072/0073/0074/0075/0077 ·
[lessons-and-resolutions-2026-07](../research/lessons-and-resolutions-2026-07.md)
(the incident evidence behind every rule above).
