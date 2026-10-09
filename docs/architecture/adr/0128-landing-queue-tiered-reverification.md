# ADR-0128: Fleet lanes land through one queue, and their overlap with the peers picks the re-verification

- **Status:** Proposed (2026-10-09). The operator chose the approach on 2026-10-09: Approach A plus two cheap parts of Approach B. The operator decided four details of it on the same day (below). Each decision moves to Accepted as its component lands ([plan](../../plans/concurrent-cycle-landing-2026-10.md) §16).
- **Spec of record:** [fleet-landing-queue.md](../fleet-landing-queue.md). It holds the rules, the codes, the config keys and the limits.
- **Plan:** [concurrent-cycle-landing-2026-10.md](../../plans/concurrent-cycle-landing-2026-10.md).
- **Research:** [concurrent-cycle-landing-2026-10.md](../../research/concurrent-cycle-landing-2026-10.md).
- **Amends:**
  - [ADR-0078](0078-fleet-landing-prefix-queue.md): the prefix queue becomes durable and gains a driver, the tiers and a stage dial. The in-memory composer and its tail trim retire.
  - [ADR-0105](0105-identity-preserving-fleet-rebase.md): a queue lane composes its audited tree, so B1 and the recovery-path rebind retire for queue lanes. B4 accepts two new record methods.
  - [ADR-0049](0049-concurrent-multi-cycle-execution.md) S5 and S5b: the tiers replace rebase-and-re-audit for fleet lanes, and the queue holds no lock across LLM work.
- **Relates to:**
  - [ADR-0039](0039-failure-floor-and-failure-signal-contract.md) §8.1: the two-phase landing, reused unchanged;
  - [ADR-0048](0048-work-conservation-fast-reland-resilient-ship.md): conserve proven work;
  - [ADR-0072](0072-system-failure-policy-and-halt.md): a red tip is a system failure;
  - [ADR-0097](0097-read-only-phase-worktree-fence.md): the writable paths of the debugger;
  - [ADR-0123](0123-ledger-durable-evidence-segments-incremental-verify.md): durable evidence;
  - [ADR-0126](0126-every-iterative-loop-converges-or-escalates.md): no third round;
  - [ADR-0127](0127-push-only-event-channels.md): the `ship.landed` event.
- **Evidence:**
  - 45 fleet-rebase recoveries in 14 days cost 24.2 min and 1.49 LLM phases each (research F3.2).
  - Cycle 1843 paid a Build and an Audit because its identity proof never ran (F2.2).
  - The prefix queue of ADR-0078 never had a production caller (F5.1).
  - Of 39 classified rebases, 18 needed no LLM phase, and 2 conflicted. The other 19 met through a package edge, an unknown path or a shared file (F3.9).
  - Compile inputs under `go/` include embedded data, and production code reads three prose roots (F4.17, F4.18).
  - No check runs on the composed tree before a landing today (F4.19).

## Context

A fleet lane forks from `main` at dispatch. When another writer lands first, the lane's ship fails with `GIT_FLEET_REBASE_NEEDED`, and a recovery ladder rebases it and checks it again. The ladder picks Build, Audit or a carry from the shape of the change: committed, pending or unwound. It does not pick from the overlap between the lane and its peers.

The operator asked on 2026-10-09: "Design a better solution to improve the situation: rebase + re-audit every time one cycle is done and another completes later." The operator then chose a landing queue with tiered re-verification, plus the derived catalog and the dispatch partition from the avoidance approach.

## Decision

1. **One landing queue.** An audited fleet lane does not merge itself. It enqueues, and the queue orders the landings by ticket.
2. **The queue decides, and the head lands.** Each lane composes, proves, verifies and lands its own candidate, at its turn.
3. **One writer to `main` at a time.** Only the head lands, inside `ship.lock`, with the two-phase landing of ADR-0039.
4. **Compose the audited tree.** `merge-tree` composes `T0` onto the tip from `base0`. Ship's inbox consumption comes at the landing.
5. **An overlap proof picks the tier.** Its inputs are the peer delta, the lane change and both import closures. Package edges count in both directions.
6. **T1, disjoint.** Keep the audit verdict, and run only the composed gates, scoped by test impact. No LLM phase runs.
7. **T2, derived only.** Regenerate with the generator of the composed tree, then continue as T1.
8. **T3, a real code overlap.** Run an interaction-only review of the two diffs, then the gates. No Build and no full Audit run.
9. **The review seat.** Opus 5.5 at medium effort, inside the Claude-family floor. A deadline bounds the review, and past it the verdict is `UNSURE`.
10. **T4, a conflict or a red gate.** Eject the candidate. The green prefix lands, and the lane goes to repair.
11. **Unknown overlap is never T1.** A failed input raises the tier to T3. So do an unowned path under `go/` and a path outside the prose allowlist.
12. **Gates follow the tree.** Compile under each tag set, and check the registries. Test what both changes reach, and the readers of the lane's paths.
13. **A red `main` tip is a system failure.** A red reruns alone, then runs on the tip. A red `main` tip pauses the queue, and the loop halts.
14. **No LLM on a prefix that has not landed.** No lock is held across LLM work.
15. **A kernel wait.** The owner takes its lock before its record is visible. A waiter wakes when the candidate ahead is terminal or dead, and it keeps no lock of another owner.
16. **The landing intent decides a dead owner in `landing`.** `complete` gives `landed`, and `prepared` pauses the queue. No intent, or an `unwound` intent, ejects the candidate.
17. **A paused queue parks its candidates.** Each owner parks its record with its ticket, releases its lock and ends its ship. No process waits through a pause, and the cycle resume re-attaches each candidate.
18. **One derived catalog.** The skill and command projections join it, and every consumer reads that one home.
19. **A narrower dispatch partition.** `PartitionGraph` keeps one file, one package and the global zone in one lane. `package` is the default.
20. **A safe fallback at dispatch.** A todo with no files keeps today's file partition and never fails a wave.
21. **Stages, not flags.** `shadow` comes first, then `enforce`. T3 starts as a full Audit, then moves to the review.
22. **Shadow measures the escapes.** The CI suite runs on each T1 or T2 tree of shadow, with its tip as the control.
23. **An interim fix.** Until the queue enforces, today's ladder runs the identity proof for continuation lanes.
24. **An honest log.** The ladder reports "the proof did not run" apart from "the proof failed".
25. **Bookkeeping is cheap.** A peer delta of bookkeeping paths only needs one gate: the tests that read the lane's paths. Predicates under `go/acs/` in it also compile under the `acs` tag. Gate results carry across it.
26. **No ship-window lease for queue lanes.** The queue orders the landings in `enforce`.

### Decided on 2026-10-09

The operator decided four details ([plan](../../plans/concurrent-cycle-landing-2026-10.md) O10 to O13):

- the dispatch partition defaults to `package`, and `closure` stays an option;
- package edges count in both directions;
- a red tip pauses the queue as a system failure under ADR-0072;
- the review runs on Opus 5.5 at medium effort, and the effort rises only after measured misses.

The console decided four more on the plan's recommendations. The operator can override each one (plan CD1 to CD4):

- the head lane lands its own candidate, and no separate composer process lands for other cycles;
- the other writers of `main` stay outside the queue, and shadow measures their compositions;
- the B1 to B5 ladder retires for fleets at stage 7, after the exit criteria hold;
- a shared path keeps its Build, and the plan measures the share before any change of the rebind.

## Alternatives considered

| Alternative | Why not |
|---|---|
| B alone: avoidance at dispatch | Each lane after the first still diverges, and B has no rule for the verdict after the rebase. Its closure rule conflicts 80.5% of package pairs. |
| C: stacked speculation | A failed lane forces a rebuild of every lane above it, and an LLM Build takes 6 to 20 min. |
| Fleet width 1 | It serializes the work, not only the landing, so throughput falls to about a third. |
| E: an external merge queue | It has no notion of an audit verdict, so each red sends the lane to a full re-audit. It also adds a second route to `main`. |
| One composer process that lands for every lane | It must run the ship of other cycles in one process, and each lane is its own process. |
| The current rule of `PartitionGraph` at dispatch | Transitive package sets meet for 80.5% of package pairs, which collapses fleet width. |
| Overlap by changed symbols | It is not sound without a call graph, and the module adds no dependency for one. |
| The patch-id carry (RUNG 0) | It never fires in worktree mode, and a patch-id hides whitespace and binary changes (ADR-0105). |
| Package edges in the lane's direction only | A lane change inside the peer's closure can break the peer. Both directions move 4 of 39 rebases to T3 (research F3.9). |
| Trust each Markdown file under `docs/` | Production code reads three prose roots (research F4.18). |
| Wake a waiter on each change of a composed ref | It needs a ref-change wake that ADR-0127 does not give yet. At width 3, the mean wait is 0.53 min (research §11). |
| On a red tip, land the candidates whose selection leaves out the red package | A red `main` is a system failure (ADR-0072), and the operator chose to pause. |
| The review at high effort | The operator rule starts each seat at medium effort, and raises it only on a measured gain. |
| Wait through a pause on a lock that `resume` releases | It needs a holder that lives through the halt, and the death of that holder wakes each waiter into a pause that still holds. A halted loop keeps no lanes alive (ADR-0072). |
| Take the predicates under `go/acs/` out of bookkeeping | It adds a package edge each time the lane changes a package that a peer's predicates import. A compile of the `acs` tag set checks them for less. |

## Consequences

- **Fewer minutes and LLM phases.** The model expects 8.8 min and 0.66 LLM phases for each rebase at stage 5, and 10.4 min at stage 6. The same 39 cycles cost 24.0 min and 1.51 today (research §11). The shadow stage measures both before enforce, and its oracle counts the escapes.
- **New parts.** The queue store under `.evolve/landing/queue/`, the overlap proof, the derived catalog, the test selection and the `landing-review` phase (plan §7).
- **Changed protected files.** Ship accepts two new record methods, and the explanation rebind drops derived outputs from its identity domain. These are console work, merged at wave boundaries.
- **Retired paths.** For queue lanes, the B1 unwind, the recovery-path rebind and carry, RUNG 0, RUNG 2 and the in-memory `PrefixQueue` retire (plan §9).
- **New operator duties.** The queue pauses on a red tip or a stranded head. A red tip halts the loop until a fix lands (ADR-0072). `evolve landing queue resume` clears the pause after the fix.
- **Accepted limits.** Package granularity, hermetic tests, imperfect selection, a review that can be wrong, and a Build for a shared path (spec, Limits).
