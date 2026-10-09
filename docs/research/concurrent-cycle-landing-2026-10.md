# Concurrent cycle landing: research dossier (2026-10)

> The plan that uses this research: [concurrent-cycle-landing-2026-10.md](../plans/concurrent-cycle-landing-2026-10.md). The spec of record: [fleet-landing-queue.md](../architecture/fleet-landing-queue.md). The decision record: [ADR-0128](../architecture/adr/0128-landing-queue-tiered-reverification.md).
> This dossier builds on [merge-concurrency-2026](merge-concurrency-2026/README.md), the [identity-preserving rebase review](2026-09-26-identity-preserving-rebase-design-review.md) and [swarm-concurrency-2026-05](swarm-concurrency-2026-05.md).
>
> The findings F2.1 to F9.3 feed the refinements R1 to R33 at the end. The plan's decisions cite them.

Date: 2026-10-09. Method:

- I checked each code claim against `main` at `4f934cf90`.
- I read the runtime runs, logs and inbox, and changed nothing there (`runtime/.evolve/`). §15 holds the steps that made the tables.
- I checked each external claim against the page at its link on 2026-10-09.
- "Synthesis" marks my own inference. "Estimate" marks a number that the cost model assumes and that the shadow stage must measure.

## Table of contents

1. [The request](#1-the-request)
2. [Incident data: cycles 1843 and 1841](#2-incident-data-cycles-1843-and-1841)
3. [The recovery census](#3-the-recovery-census)
4. [The current ladder](#4-the-current-ladder)
5. [Why the prefix queue is off](#5-why-the-prefix-queue-is-off)
6. [Prior art: merge queues and gates](#6-prior-art-merge-queues-and-gates)
7. [Prior art: agent harnesses](#7-prior-art-agent-harnesses)
8. [Prior art: semantic conflicts and test selection](#8-prior-art-semantic-conflicts-and-test-selection)
9. [Git mechanics](#9-git-mechanics)
10. [Adopt or reject](#10-adopt-or-reject)
11. [The cost model](#11-the-cost-model)
12. [The approaches](#12-the-approaches)
13. [The winner](#13-the-winner)
14. [Refinements](#14-refinements)
15. [Reproduction](#15-reproduction)

## 1. The request

The operator wrote this on 2026-10-09:

> "Design a better solution to improve the situation: rebase + re-audit every time one cycle is done and another completes later. Consider multiple cycles running at the same time and conflict. Research how other agent/harness solutions solve this. Propose at least 2 approaches and pick the winner."

The operator also chose the winner: Approach A plus two cheap parts of Approach B ([plan](../plans/concurrent-cycle-landing-2026-10.md) §4). This dossier records the evidence for that choice. It also records the evidence for the details that the choice leaves open.

## 2. Incident data: cycles 1843 and 1841

**F2.1** Cycle 1843 rebased cleanly, but its identity proof never ran. Its ship failed with `GIT_FLEET_REBASE_NEEDED` at 03:24:46Z, because cycle 1844 landed `b7d31e54f` at 03:21:50Z. The loop log then shows two lines in this order:

- "cycle 1843 ship unwind declined: consumed item 2026-09-30T19-12-01Z-route-verbs-refuse-mid-wave.json released a continuation that only ship's commit records"
- "cycle 1843 rebased change is not proven identical and pending on b7d31e54fb7e448bf35740f7264308bef0261756; Build re-authors the explanation"

Source: `runtime/.evolve/logs/20261009T021848Z/loop.log`, lines 353 and 355.

**F2.2** Four code facts make the chain:

1. Cycle 1843 continued failed cycle 1841, so its consumed inbox item carries `released_continuations`.
2. The B1 unwind declines for such an item (`go/internal/core/ship_recovery_unwind.go:125-135`).
3. With no unwind, the rebase replays ship's commit, so `HEAD` is not the fork point.
4. `rebindPendingChange` then returns `(false, nil)` and never calls the proof (`go/internal/core/ship_recovery.go:184-190`).

The default branch then prints "not proven identical" and routes to Build (`ship_recovery.go:171-179`). The consumed item at commit `61ec214b6` holds the released continuation of cycle 1841.

*Implication:* the log line is false. The proof did not fail; it did not run. The defect is a precondition on the shape of the change (committed or pending), not on its content.

**F2.3** The checks of the proof pass on these inputs (synthesis). Cycle 1844 changed 10 paths and cycle 1843 changed 10 paths, and no path is in both. Neither side changed `.gitattributes` or `.gitignore`. Those are the checks of `lineageHolds` (`go/internal/explanationdocs/rebind.go:136-151`), and a clean replay keeps the bytes of each lane path. With the proof held, the B3 carry runs the full composed gates (11.3 min on average, F3.4). If they pass, the lane ships with no LLM phase.

*Implication:* cycle 1843 paid a Build of 16.8 min and an Audit of 5.0 min that the evidence did not need. Its recovery took 30.6 min from the ship error to the landed ship.

**F2.4** The same class cost cycles 1801 and 1818. Their logs show the same decline, for items that released a continuation (`2026-09-26T20-41-00Z-gc-reaps-before-its-refusal.json` and `2026-09-26T10-10-00Z-policy-resolver-hygiene.json`). Since 2026-10-06, these three cycles are all of the Build-route recoveries: 24.6 + 47.6 + 30.6 = 102.8 min and 7 LLM phases.

*Implication:* each continuation lane that meets a fleet rebase pays a Build and an Audit. The landing queue removes the class, because it composes the audited tree and not ship's commit ([spec](../architecture/fleet-landing-queue.md) §5). The plan also adds an interim fix for the current ladder (plan component Q12).

**F2.5** Cycle 1841 spent 61.6 min after its ship error, and it did not ship. Source: `runtime/.evolve/runs/cycle-1841/signals.ndjson` and `phase-timing.json`.

| From (UTC) | To | Step | Minutes |
|---|---|---|---|
| 23:15:51 | 23:27:24 | unwind, rebase, identity proof, full composed gates | 11.6 |
| 23:27:24 | 23:45:09 | Audit 2: FAIL on "skill projection drift" | 17.7 |
| 23:45:09 | 23:53:08 | Build repair 1 | 8.0 |
| 23:53:08 | 23:58:40 | Audit 3: FAIL on the same drift | 5.5 |
| 23:58:40 | 00:06:19 | Build repair 2 | 7.6 |
| 00:06:19 | 00:13:46 | Audit 4: FAIL on the same drift; the repair budget is spent | 7.5 |
| 00:13:46 | 00:17:26 | retro, then the seal (FAIL) | 3.7 |

**F2.6** Two faults drove cycle 1841, and neither one is an interaction between the two changes.

- The unwind and the proof worked: the log says "rebase is byte-identical". The full composed gates then failed on `apicover`. The recorded tail ("…proctree 2.940s coverage: 100.0% of statements FAIL make: *** [apicover-enforce] Error 1") does not name the failing package.
- All three audits failed on a false red of the skills drift gate. The gate graded the lane with the generator of the host binary, while the composed tree carried the new generator of peer cycle 1842. Commit `10d0c1d39` fixed the gate, and it calls 1841 "the third false red of this class (1828, 1840, 1841)".

*Implication:* the 1841 cost came from a full-suite gate run that named no culprit, and from a gate that used the wrong generator.

**F2.7** The skill projections are not in the derived catalog. `derivedArtifacts` holds one entry, `docs/architecture/control-flags.md` (`ship_recovery.go:236-241`). A conflict on a `commands/*.md` stub or in a `SKILL.md` phase-facts region is thus a "non-derived conflict" that goes to the debugger (`ship_recovery.go:368-372`).

*Implication:* the B1 part of the operator decision adds them. It closes the neighbour class of 1841: a real conflict or a real staleness of a generated projection after a composition. It does not explain the red of 1841 itself, which was false.

## 3. The recovery census

**F3.1** The retained runs hold 46 ship errors with `GIT_FLEET_REBASE_NEEDED`, from cycle 1691 (2026-09-25) to cycle 1843 (2026-10-09). A recovery ran for 45 of them, and 39 lanes shipped. Source: each `runtime/.evolve/runs/cycle-*/phase-timing.json`, with step 1 of §15.

**F3.2** A recovery costs 24.2 min on average (median 21.8) and 1.49 LLM phases. The total is 1,091 min, which is 18.2 hours in 14 days. Recovery minutes run from the ship error to the end of the next ship that passed, or to the end of the last recovery phase. LLM phases count Build, Audit, debugger and TDD outcomes in that window.

| Route today | Recoveries | Mean (min) | Median (min) | LLM phases (mean) | Shipped |
|---|---|---|---|---|---|
| Build | 14 | 29.0 | 28.0 | 2.29 | 13 |
| Audit | 17 | 19.0 | 17.9 | 1.24 | 15 |
| Ship (a carry record was written) | 11 | 25.9 | 29.5 | 0.64 | 10 |
| Debugger (a conflict) | 3 | 25.8 | 14.7 | 2.33 | 1 |
| All | 45 | 24.2 | 21.8 | 1.49 | 39 |
| Since 2026-10-06 | 10 | 30.4 | 27.1 | 1.40 | 8 |

**F3.3** Only three recoveries carried the audit verdict with no LLM phase: cycles 1829, 1832 and 1834. They still took 15.2 to 21.8 min, because the full composed gates took 11.4 to 15.1 min.

*Implication:* a carry that works costs more today than a full Audit (median 5.7 min, F3.6).

**F3.4** 21 recoveries ran the full composed gates, for 11.3 min on average. The gates are four whole-repo `make` targets, run in sequence: `build`, `test`, `test-acs-durable` and `apicover-enforce` (`go/cmd/evolve/cmd_composition_wiring.go:71-115`). The set is `ciparity.RequiredComposedGates` (`go/internal/ciparity/composedgates.go:13`), and a red gate declines the carry (`go/internal/core/composition_carryforward.go:46-87`).

**F3.5** The full gates declined the carry 10 times. Cycles 1735, 1742, 1744, 1745, 1748 and 1760 failed on `test` and `apicover`. Cycles 1769, 1794, 1796 and 1841 failed on `apicover` alone. Eight of these lanes then shipped the same composed tree after one Audit, with no Build. Source: the loop log lines "identity carry declined: composed-tree gates not green".

*Implication:* most of these reds named no defect that the Audit or the ship found. Synthesis: a whole-suite run on a host with three busy lanes meets flaky reds and environment reds. A scoped run selects fewer unrelated tests, and a flake screen separates a flake from a real red.

**F3.6** These are the phase medians for PASS outcomes over cycles 1600 to 1899. Ship takes 2.8 min (p90 5.5), Audit 5.7 min (p90 8.3) and Build 6.3 min (p90 20.0). Source: `phase-timing.json`, with n = 96, 134 and 190.

**F3.7** The ship's importer backstop already runs a scoped test set: `go test -count=1` over the importer closure of the changed packages (`go/internal/phases/ship/repocontract_importers.go:48-95`). Over 198 lane runs in the loop logs, it took a median of 74 s (p90 187 s). Runs with 10 targets or fewer took a median of 2 s. Runs with more targets took a median of 170 s, with a maximum of 449 s.

*Implication:* a test set scoped by impact costs seconds to about 3 min, against 11.3 min for the full gates.

**F3.8** Main has more writers than the lanes. Since 2026-10-01, the first-parent history of `main` holds 119 commits. They are 53 dossier closeouts, 36 cycle landings, 16 merges of `origin/main`, 12 inbox stamp commits, 1 release and 1 pull request merge. A dossier closeout adds only `knowledge-base/cycles/cycle-N.json` and `cycle-N.md`.

*Implication:* each such commit makes every lane in flight diverge. In 7 of the 39 classified cycles, the peer delta held only bookkeeping paths: 1698, 1701, 1704, 1712, 1715, 1727 and 1832.

**F3.9** I classified the 39 cycles that have a landing record with the zones and the tier rules of the [spec](../architecture/fleet-landing-queue.md) §6 to §8. The package map and the import graph are the union of the four tag sets at `4f934cf90` (§15, steps 2 and 3).

| Tier | Cycles | What fired |
|---|---|---|
| T1, by a bookkeeping-only peer delta | 7 | `bookkeeping_peer` |
| T1, disjoint | 11 | no rule |
| T2 | 0 | a derived entry fired in 5 cycles, and each of them was also T3 |
| T3, no shared path | 17 | a package edge in 16 cycles; unknown paths in 2 (1730 and 1748) |
| T3, a shared path | 2 | `shared_path` (1731 and 1801) |
| T4 | 2 | a genuine conflict (1719 and 1792) |

- Under a file-only rule, 35 of the 39 are disjoint.
- With the edges of the lane's own closure only, T1 is 21, T2 is 1 and T3 is 15. The edges in both directions move 4 cycles to T3, an operator decision (plan O11).
- The import graph of the default tags alone gives the same tiers.
- The unowned paths of F4.17 raise one cycle, 1730, to T3. Under a rule that maps a file to the package of its nearest directory, 1730 was T2.
- No path of the 39 cycles is under a read root of F4.18.
- One bookkeeping-only peer delta, of cycle 1832, holds Go code: the predicates of cycles 1723 and 1831 under `go/acs/`.

The tier of each cycle, beside the cost of its recoveries today (step 1 of §15):

| Cycle | Tier | Route today | Minutes today | LLM phases today | Evidence |
|---|---|---|---|---|---|
| 1691 | T3 | build | 28.2 | 2 | 3 package edges, first lane to peer at `internal/adapters/statemap`; derived entry `signal-codes` fired |
| 1692 | T1 | build | 13.7 | 2 | none |
| 1694 | T1 | build | 18.9 | 2 | none |
| 1698 | T1 | build | 26.1 | 2 | a bookkeeping-only peer delta |
| 1701 | T1 | build | 46.5 | 3 | a bookkeeping-only peer delta |
| 1702 | T1 | build | 34.6 | 5 | none |
| 1704 | T1 | build | 12.9 | 2 | a bookkeeping-only peer delta |
| 1712 | T1 | audit | 7.6 | 1 | a bookkeeping-only peer delta |
| 1715 | T1 | audit | 5.3 | 1 | a bookkeeping-only peer delta |
| 1717 | T1 | audit | 17.0 | 1 | none |
| 1719 | T4 | debugger, then build | 30.4 | 5 | a genuine conflict |
| 1722 | T3 | audit | 8.6 | 1 | 1 package edge, first peer to lane at `internal/dashboard` |
| 1724 | T3 | audit | 14.1 | 1 | 1 package edge, first lane to peer at `internal/commentaudit` |
| 1727 | T1 | audit | 9.5 | 1 | a bookkeeping-only peer delta |
| 1730 | T3 | audit | 8.3 | 1 | 2 unknown paths, first `go/internal/rawgitratchet/baseline.json` |
| 1731 | T3 | build | 34.1 | 2 | shared `go/internal/sizeratchet/offenders.json`; 1 package edge, first peer to lane at `internal/failurelog`; 1 unknown path, first `go/internal/sizeratchet/offenders.json` |
| 1737 | T3 | build | 27.9 | 2 | 3 package edges, first lane to peer at `internal/dashboard` |
| 1742 | T3 | audit | 22.1 | 1 | 2 package edges, first lane to peer at `internal/cyclecost` |
| 1744 | T1 | audit | 21.2 | 1 | none |
| 1745 | T1 | audit | 19.6 | 1 | none |
| 1748 | T3 | audit | 23.5 | 1 | 1 package edge, first lane to peer at `internal/router`; 2 unknown paths, first `go/internal/rawgitratchet/baseline.json` |
| 1760 | T1 | audit | 18.4 | 1 | none |
| 1766 | T1 | ship | 31.2 | 1 | none |
| 1768 | T3 | ship | 18.5 | 1 | 1 package edge, first peer to lane at `internal/phasecoherence` |
| 1769 | T1 | audit | 17.3 | 1 | none |
| 1772 | T3 | ship | 31.3 | 1 | 4 package edges, first lane to peer at `internal/addedtests` |
| 1773 | T3 | ship | 30.7 | 1 | 3 package edges, first lane to peer at `cmd/evolve` |
| 1782 | T3 | ship | 30.0 | 1 | 7 package edges, first lane to peer at `cmd/evolve`; derived entry `signal-codes` fired |
| 1792 | T4 | debugger | 59.6 | 3 | a genuine conflict |
| 1794 | T3 | audit | 26.0 | 1 | 2 package edges, first lane to peer at `internal/releasepreflight` |
| 1796 | T1 | audit | 17.9 | 1 | none |
| 1801 | T3 | build | 24.6 | 2 | shared `docs/operations/runtime-reference.md`; 1 package edge, first lane to peer at `internal/evalgate`; 2 unknown paths, first `skills/commit/SKILL.md` |
| 1810 | T3 | ship | 29.5 | 1 | 1 package edge, first lane to peer at `internal/inboxbatch` |
| 1818 | T3 | build | 47.6 | 3 | 1 package edge, first lane to peer at `internal/profiles`; derived entry `skill-projections` fired |
| 1824 | T3 | ship | 39.6 | 1 | 1 package edge, first peer to lane at `internal/interaction` |
| 1829 | T3 | ship | 21.8 | 0 | 2 package edges, first peer to lane at `internal/phases/ship`; derived entry `signal-codes` fired |
| 1832 | T1 | ship | 15.5 | 0 | a bookkeeping-only peer delta |
| 1834 | T1 | ship | 15.2 | 0 | none |
| 1843 | T3 | build | 30.6 | 2 | 3 package edges, first lane to peer at `cmd/evolve`; derived entry `signal-codes` fired |

*Implication:* 18 of the 39 rebases need no LLM phase at all. 17 need one interaction review, not a Build and a full Audit. Only the 2 shared-path cycles and the 2 conflicts need a Build. Cycle 1829 shows the cost of a stricter rule: today's carry landed it with no LLM phase, and the queue gives it T3.

**F3.10** Four of the 39 pairs share a Go package: cycles 1773, 1782, 1792 and 1843, all in `go/cmd/evolve`. Cycle 1792 also had a textual conflict.

*Implication:* a dispatch rule that keeps two todos of one package apart removes 4 of 39 pairs from a wave. Its cost in width is small.

## 4. The current ladder

**F4.1** The raise point. `Landing.CheckFastForward` asks `merge-base --is-ancestor`. Under a fleet, a "no" becomes `GIT_FLEET_REBASE_NEEDED` with the class transient (`go/internal/phases/ship/landing/integrate.go:28-34`, `:57-67`).

**F4.2** The hook. `recoverShipError` sends a ship error to `recoverFromShipError`, keeps the worktree and moves the loop cursor (`go/internal/core/retry_opts.go:113-141`).

**F4.3** The budget. A contention code gets `max(2, fleet width + 1)` tries, after a jittered backoff whose window grows from 2 s to 30 s (`go/internal/core/ship_recovery_budget.go:16-66`; `maxRecoveryDepth = 2` at `retry_backoff.go:80`).

**F4.4** The ladder (`go/internal/core/ship_recovery.go:17-140`) has these steps:

- A pre-screen classifies the candidate as landed, clean or conflict (`:46-73`).
- B1 unwinds ship's commit to the audited tree (`:74`, `ship_recovery_unwind.go`).
- `rebaseWithDerivedRegen` replays the change, and it regenerates conflicts in `control-flags.md` only (`:335-397`).
- For a cycle with an explanation contract, `routeRebasedExplanation` runs the B2 rebind and the B3 carry (`:146-180`). It returns Ship, Audit or Build.
- RUNG 0 and RUNG 2 run only for a cycle with no contract (`:96-107`).
- A conflict becomes `GIT_FLEET_REBASE_CONFLICT` and goes to the debugger (`:108-112`).

**F4.5** Every cycle takes the contract path today ([ADR-0105](../architecture/adr/0105-identity-preserving-fleet-rebase.md) Context). RUNG 0 and RUNG 2 are thus unreachable in practice. RUNG 2 also has no production reviewer: nothing outside tests calls `WithScopedMergeReviewer`.

**F4.6** The router rule `fleet-rebase-reaudit` sends `GIT_FLEET_REBASE_NEEDED` to Audit (`go/internal/router/recovery.go:99-110`). The contract path returns before the router, so the rule serves only a cycle with no contract.

**F4.7** The ship-window lease serializes only the window from the audit binding to the push (`go/internal/core/cyclerun_record.go:38-57`, `go/internal/shipwindow/shipwindow.go`). A lane forks from `main` at dispatch. Each landing between the fork and the lane's own ship makes the lane diverge.

*Implication:* the lease prevents `AUDIT_BINDING_HEAD_MOVED`, not `GIT_FLEET_REBASE_NEEDED`. In a wave of width 3, each lane that lands after another lane diverges.

**F4.8** `ship.lock` serializes only the git critical section of one ship try (`go/internal/phases/ship/gitops.go:22-32`). [ADR-0049](../architecture/adr/0049-concurrent-multi-cycle-execution.md) S5b deferred a merge-queue lock, because a lock held across a "multi-minute re-audit" is "a liveness SPOF".

*Implication:* a queue must hold no lock across LLM work.

**F4.9** The dispatch partition is file-only. `loopwave.dispatch` calls `fleet.PlanFromTriage`, which calls `PlanCycles` and `Partition` (`go/internal/loopwave/dispatch.go:85`, `go/internal/fleet/partition.go:11-64`). `PartitionGraph` has no production caller (`go/internal/fleet/packagegraph.go:68`).

**F4.10** A todo with no declared files gets `Files: [id]`, a path that exists nowhere, so it never collides (`go/internal/fleet/triageplan.go:73-76`). `PartitionGraph` fails on such a todo, because `TransitivePackageSet` runs `os.Stat` on each file (`packagegraph.go:20-22`). In the lane triage decisions of cycles 1700 to 1899, 155 of 155 `top_n` cards carry `files`. One of 130 decisions used `committed_floors`, which carry no files.

*Implication:* the fallback is rare, but it must never fail a wave.

**F4.11** `PartitionGraph` makes two todos conflict when their transitive package sets meet. Over the 190 module packages that import another module package, that rule conflicts 80.5% of all pairs. The directed rule (a package of one side is in the closure of the other side) conflicts 26.4%. `go/cmd/evolve` alone reaches 242 of 271 packages. Source: `go list` at `4f934cf90` (1.8 s), with step 6 of §15.

*Implication:* `PartitionGraph` as it is collapses fleet width. A shared leaf import, such as `internal/sysexec`, which 140 packages import directly or through others, is not an interaction.

**F4.12** Test impact code exists. `changedpkgs.ImporterClosureChecked` returns the packages whose build or test binary links a changed package (`go/internal/changedpkgs/importerclosure.go:33-81`). `regressiontia` computes a shadow selection over the ACS regression corpus, and its fail-safes "resolve toward RUNNING a predicate" (`go/internal/regressiontia/regressiontia.go:1-21`).

*Implication:* the composed gates reuse these two packages. No second selector is necessary.

**F4.13** The repo-contract pack at ship runs a fixed pack of 14 package patterns, an added-test backstop and an importer backstop. It reruns a failure alone (`go/internal/repocontract/repocontract.go:21-38`, `go/internal/phases/ship/repocontract_importers.go:48-95`). It runs no `apicover` and no ACS regression.

**F4.14** These are the generated projections in the tree. Source: a search for the `GENERATED` markers.

| Projection | Shape | Generator | Check |
|---|---|---|---|
| `docs/architecture/control-flags.md` | region `GENERATED:flag-index` | `evolve flags generate` | `evolve flags check` |
| `docs/architecture/signal-codes.md` | region `GENERATED:signal-codes` | `evolve signals codes generate` | `evolve signals codes check` |
| `skills/<phase>/SKILL.md` (8 files) | region `GENERATED:phase-facts` | `evolve skills generate` | `evolve skills check` |
| `commands/*.md` (32 files) | whole file | `evolve skills generate` | `evolve skills check` |
| `.codex-plugin/plugin.json` and `.agents/plugins/marketplace.json` | whole file | `evolve skills generate` | `evolve skills check` |
| `agents/evolve-router.md` | region `GENERATED:goal-recipes` | none | `go/internal/router/recipes_drift_test.go` |

**F4.15** The explanation contract treats `docs/`, `knowledge-base/`, `.evolve/inbox/`, `.evolve/evals/` and `go/acs/` as non-material (`go/internal/explanationdocs/explanationdocs.go:518-545`). `commands/`, `skills/` and `agents/` are material. The diff digest of the rebind covers every changed path (`rebind.go:85-110`).

*Implication:* a regenerated `commands/*.md` stub breaks the byte identity of the lane's change. T2 thus needs the derived outputs out of the identity domain (plan component Q9).

**F4.16** Today's carry does not re-run the lane's own predicates on the composed tree. `identityCarryForward` runs the four `make` targets only (`go/internal/core/identity_carry_forward.go:47-54`). Ship checks the predicate receipt against the audited tree of the audit row (`go/internal/phases/ship/audit.go:332-345`). [ADR-0105](../architecture/adr/0105-identity-preserving-fleet-rebase.md) B3 says that "the host predicate suite re-runs on `T1` with a sealed receipt".

*Implication:* the code and the ADR disagree. The queue re-runs the lane's predicates on the composed tree and binds a fresh receipt to it ([spec](../architecture/fleet-landing-queue.md) §9).

**F4.17** Compile inputs under `go/` are not only `.go` files. These are the tracked files under `go/`, over the four tag sets at `4f934cf90` (§15, step 2):

| Class | Files | Examples |
|---|---|---|
| named in a file list of `go list` | 4,689 | 21 of them are `EmbedFiles`, such as `go/internal/bridge/manifests/*.json` and `go/internal/skillcheck/templates/skill.md.tmpl` |
| in a `testdata/` tree of a package | 210 | test fixtures |
| bookkeeping (`go/acs/cycle*/`) | 548 | the predicates of each cycle |
| the global zone | 45 | `go/go.mod`, `go/Makefile`, and the 39 files of `go/vendor/` |
| owned by no package | 12 | `go/internal/sizeratchet/offenders.json`, `go/README.md` |

- The module builds its two dependencies from `go/vendor/`: `go list` resolves `gopkg.in/yaml.v3` to `go/vendor/gopkg.in/yaml.v3`.
- The embedded manifests are the tier tables of the CLI families. A change to one changes `internal/bridge` with no change to a `.go` file.

*Implication:* a rule by the `.go` suffix misses embedded data. A rule by the nearest directory gives a package the data that no package owns. The proof maps each path through the file lists of `go list` and the `testdata/` trees. A path that no package owns is unknown, and `go/vendor/` joins the build zone ([spec](../architecture/fleet-landing-queue.md) §6, §7.3).

**F4.18** Production code reads data files that no catalog lists. A search of the non-test Go files for path literals under `docs/` finds three reads of Markdown:

- `stelint.LoadStandard` reads the word table of `docs/conventions/ste100-writing.md` (`go/internal/stelint/stelint.go:58-66`). The STE floor of the build calls it on the worktree (`go/internal/core/ste_floor.go:33`).
- The default search roots of `internal/research` include `docs/research/` (`go/internal/research/kb.go:68-75`), and task recall searches them (`go/internal/core/task_recall.go:28`).
- The installer requires three files under `docs/reference/` (`go/internal/installer/installer.go:73-77`, `:159`).

The other literals under `docs/` are path classifiers, write targets, derived outputs or text in messages. Production code also reads `docs/architecture/phase-registry.json`, `.evolve/phases/`, `.evolve/profiles/`, `skills/` and `agents/` (`go/internal/phasespec/mergedcatalog.go:10-60`, `go/internal/skillcheck/skillcheck.go:301-374`). It reads the bookkeeping root `knowledge-base/cycles/` too (`go/internal/dossier/cyclesdir.go:14`). A bookkeeping file is new, and its name is unique to its cycle or item. So a composition does not change a file that the lane read.

*Implication:* a rule that trusts all Markdown under `docs/` is wrong for these three roots. The proof treats them as unknown, and each other path outside the prose allowlist is unknown too. A catalog of production reads, with a guard test, is later work. The rule costs nothing on the sample: no path of the 39 cycles is under a read root (F3.9).

**F4.19** No check runs on the composed tree before a landing today. These checks run on other trees:

- The lane's audit runs on `base0` + `L`. Its CI-parity gate runs `go vet ./...` and the ACS regression tier (`go/internal/phases/audit/ciparitygate/command.go:17-25`). It runs the integration tier with `-race` over the changed packages, less the `acs` packages and the env-exclusive packages (`tier.go:19`, `tierscope.go:11-27`). It runs `apicover -enforce` over the enforced packages that the cycle changed (`go/internal/ciparity/ciparity.go:39`). The predicates run with a receipt on `T0` (F4.16).
- Ship runs on the tree that it lands: the fixed pack, the added-test backstop and the importer backstop with the default tags (F4.13).
- The required CI runs on `main` after each push (`.github/workflows/required.yml`). Its Go suite runs `go vet`, `make build`, `make test-integration`, `make test-e2e`, `make apicover-check` and `make cover-strict`, on Ubuntu and macOS (`go.yml:23-120`).
- The required CI picks the Go suite by path. A push of only `docs/reports/`, `docs/research/` or `docs/private/` Markdown, `landing/` or `docs/explain/` skips it (`required.yml:60-70`). The ACS regression tier and the plugin checks run on each push (`ci.yml:37-66`).

*Implication:* when T1 skips the Audit, the queue must run again on `C` each check whose inputs both changes reach. That is the scoped gate set of the [spec](../architecture/fleet-landing-queue.md) §9. e2e, `cover-strict` and the integration-tagged tests of the importers stay after the landing, as they are for each landing today.

**F4.20** A second registration of a signal code is not a build error. `RegisterCode` records a second registration with another module or another doc as a conflict, and it does not panic (`go/internal/signalcenter/registry.go:56-79`). `evolve signals codes check` checks only the drift of the generated region (`go/cmd/evolve/cmd_signals.go:45-75`). The one check of the whole registry is a test of `cmd/evolve`, whose binary links each producer module (`go/cmd/evolve/cmd_cycle_signal_center_test.go:167-176`).

*Implication:* two lanes that register the same code in two packages each pass alone. Their composition has a conflict that only the test of `cmd/evolve` sees. The `registries` gate runs the checks with the generator of `C`, and plan component Q11 makes the codes check fail on a conflict.

## 5. Why the prefix queue is off

**F5.1** Nothing calls it. `ship.PlanLanding` and `ship.LandPrefixes` are the only callers of `fleet.PrefixQueue`. Only tests and the ACS predicates of cycles 975, 981 and 982 call those two seams. Source: a search of the Go code outside `_test.go` files. Thus `fleet.landing=prefix-queue` changes no behavior today.

**F5.2** The history, from the git log of `prefixqueue.go`, `planlanding.go` and `landprefixes.go`, and from the processed inbox:

| Date | Event |
|---|---|
| 2026-07-13 | The merge research proposes the queue (inbox item `prefix-speculation-landing-queue`, weight 0.93). |
| 2026-07-20 | Cycle 975 builds `PrefixQueue` with no production caller. EGPS refuses the ship: "goal floor: no inert API, wiring proof required". |
| 2026-07-20 | The follow-up item `prefix-queue-ship-wiring-and-policy` asks for a caller in the ship main-push path, two review fixes and a wiring proof. |
| 2026-07-21 | Cycles 981 and 982 (`c11f30a74`, `1325b94db`) add the policy key, the seams `PlanLanding` and `LandPrefixes`, and a mutex. The item is consumed, but no caller and no verify function land. |
| 2026-07-28 | Cycle 1144 writes ADR-0078 as a backfill. It keeps `per-lane` as the default for "the unsoaked strategy". |

The item asked for a test that "the native gate-set runner drives ResolveCulprit via the verify seam". No such test exists.

**F5.3** These structural gaps kept the queue off:

| Gap | Evidence |
|---|---|
| No home across processes. The queue lives in memory, but each lane is its own `evolve cycle run` process. | `go/cmd/evolve/cmd_fleet.go:122-138` |
| No verify. Nothing composes candidate trees or runs the gates for `ResolveCulprit`. | F5.1 |
| No verdict rule. The queue re-runs gates, but the audit verdict after a composition stayed with the recovery ladder. | §4 |
| Overlap by file equality. `groups()` compares file lists only. | `go/internal/fleet/prefixqueue.go:73-111` |
| Ejection of innocent lanes. `ResolveCulprit` trims the tail until the union is green: "the ejected lane is positional, not necessarily the culprit". | `prefixqueue.go:145-152` |
| No ejection route. An ejected lane has no path to a repair. | `go/internal/phases/ship/landprefixes.go` |
| A lock across LLM work. A single composer that re-audits holds the main path for minutes, which ADR-0049 S5b rejected. | F4.8 |

**F5.4** Four docs describe the queue as live. `docs/architecture/packages/internal-phases-ship.md:44` calls `LandPrefixes` "the live driver". The `fleet.landing` entries of `runtime-reference.md` and `control-flags.md` say that `prefix-queue` "hands the SOLE main-push path" to the composer. `internal-fleet.md` says that the package "composes PASS lanes for landing".

*Implication:* the new design must add these six parts:

- a durable home;
- a real verify;
- a verdict rule;
- an overlap by import closure;
- positional ejection with a repair route;
- no lock across LLM work.

This lane adds one status sentence to each of the four docs.

## 6. Prior art: merge queues and gates

**F6.1** Zuul tests "each change applied to the tip of the branch exactly as it is going to be merged". Its dependent pipeline tests a change together with the changes ahead of it. If one fails, "changes that were expecting it to succeed are re-tested without the failed change". In the worst case, "changes are tested one at a time". Source: https://zuul-ci.org/docs/zuul/8.0.0/gating.html

*Implication:* adopt the prefix composition and positional ejection. At width 3, the worst case (serial) is our normal case.

**F6.2** The GitHub merge queue makes a `merge_group` from "the latest version of the `base_branch`" and the pull requests ahead, on a branch with the prefix `gh-readonly-queue/{base_branch}`. A pull request with "failed required status checks or conflicts with the base branch" leaves the queue, and the queue makes the branch again without it. "Build concurrency" caps the parallel CI runs between 1 and 100. Source: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue

GitHub's own engineering moved its monorepo to the queue before general availability, "shipping 30,000+ pull requests with their associated 4.5 million CI runs". Source: https://github.blog/engineering/engineering-principles/how-github-uses-merge-queue-to-ship-hundreds-of-changes-every-day/ (2024-03-06)

*Implication:* adopt the queue-owned composition and the removal on a red check. Reject one full CI run for each group: our expensive step is an LLM audit, not the tests.

**F6.3** Bors enforces the "Not Rocket Science Rule": "automatically maintain a repository of code that always passes all the tests". Serial checks limit throughput: with 15-minute CI, a Bors-style queue merges "4 changes an hour or 32 changes in an 8-hour work day". Source: https://graphite.com/blog/bors-google-tap-merge-queue

*Implication:* a serial landing is affordable at our rate (36 cycle landings in 9 days), if each landing step is short. The step is short only when it needs no LLM.

**F6.4** Uber's SubmitQueue says: "When two changes affect two disjoint sets of packages, they are independent; otherwise, they are conflicting". It lands independent changes "regardless of the order". Its proof for an order change assumes that "the builds are hermetic" and that "tests are deterministic". Sources: https://www.uber.com/en-US/blog/bypassing-large-diffs-in-submitqueue/ and https://github.com/uber/submitqueue/

*Implication:* adopt build-graph independence for test selection, with the same two assumptions. Our gates re-run only the packages that both changes affect.

**F6.5** Trunk tests "independent changes in parallel from the impacted targets each PR touches". On a red batch, "automatic bisection isolates the culprit and the healthy PRs keep moving". Also, "a PR that fails on a flake stays in line while downstream PRs test". Source: https://trunk.io/merge-queue . The page https://docs.trunk.io/merge/how-does-it-work now goes to a login page, so this dossier does not cite it.

**F6.6** Gerrit copies the Code-Review vote on `changekind:TRIVIAL_REBASE`: the same diff, after a rebase that "did not require git to perform any conflict resolution". It is the default for the Code-Review label. Source: https://gerrit-review.googlesource.com/Documentation/config-labels.html

*Implication:* review verdicts follow the change, and gates follow the tree ([merge-concurrency-2026](merge-concurrency-2026/README.md), finding 1). T1 keeps the verdict, and it is stricter than Gerrit: it also needs a disjoint import closure.

**F6.7** Rust rollups mark pull requests `always`, `maybe`, `iffy` or `never`. `iffy` is for changes to CI or to the bootstrap, and `never` keeps a change out of every rollup. On a red rollup, the culprit is unapproved and the rollup is made again without it. Source: https://forge.rust-lang.org/release/rollups.html

*Implication:* keep the solo slot of ADR-0078 for an `iffy` candidate, which is one that touches the global zone or the protected surface.

**F6.8** Google TAP runs at presubmit "the minimal set" of tests "downstream from any change", from "the global dependency graph". "A change that passes the presubmit has a very high likelihood (95%+) of passing the rest of the tests." Postsubmit splits a failing batch "into individual changes". Source: https://abseil.io/resources/swe-book/html/ch23.html

*Implication:* adopt selection by the dependency graph before the landing, with the CI on `main` as the postsubmit backstop. A scoped selection is not perfect, so measure the escapes.

**F6.9** The Chromium CQ retries a failed shard with the patch, then reruns the suite "(without patch)". A test fails the build only if it failed twice with the patch and passed without it. If it also fails without the patch, the CQ assumes that "the test is broken/flaky on tip of tree". Source: https://chromium.googlesource.com/chromium/src/+/HEAD/docs/infra/cq.md

*Implication:* adopt this flake screen for a red composed gate. A red without the candidate is a red base, not a red candidate.

## 7. Prior art: agent harnesses

**F7.1** Xu, Subramanian and Karthik studied 33,596 agent pull requests in 2,807 repositories. They replayed three-way merges of 747 co-active pairs. The textual conflict rate was 41.7% for pairs from two agents and 19.8% for pairs from one agent. The 95% confidence intervals do not overlap.

Source code held 84.4% of the conflicted files, and nearly 42% of the conflicts were structural. The authors call the rates "conservative lower bounds", because they did not measure build or semantic conflicts. Source: https://arxiv.org/abs/2607.04697

*Implication:* our file-disjoint dispatch keeps textual conflicts at 2 of 39 pairs (F3.9). The remaining risk is in the build and semantic layers, which the study did not measure.

**F7.2** AgenticFlict holds 142K+ agent pull requests from 59K+ repositories. A deterministic merge simulation processed 107K+ of them and found a conflict rate of 27.67%, with 336K+ conflict regions. Source: https://arxiv.org/abs/2604.03551

**F7.3** "Passes Alone, Fails Together" (EXPRESS '26) studies patches that "work alone but fail when merged". This "happens when one agent changes an interface or rule that another agent still relies on". On 417 mined Django pairs (834 runs), "only one showed interference". On constructed tasks with 12 Django helpers, "interference occurred in 97% of runs". Also, "a message describing the completed concurrent change recovered 82% of runs". Source: https://arxiv.org/abs/2609.25396

*Implication:* real disjoint pairs rarely interfere, which supports T1. When code is coupled, an agent that sees the concurrent change avoids most failures. That supports a T3 review whose input is the landed diff of the peer.

**F7.4** Claude Code agent teams share a task list, and "task claiming uses file locking to prevent race conditions". The docs advise: "Two teammates editing the same file leads to overwrites. Break the work so each teammate owns a different set of files." Subagents can run in their own git worktrees. Sources: https://code.claude.com/docs/en/agent-teams and https://code.claude.com/docs/en/worktrees

*Implication:* isolation and file ownership are the documented controls. Neither page defines a check after a peer lands.

**F7.5** OpenAI Codex runs "each task ... in its own cloud sandbox environment, preloaded with your repository". After a task, you can "review the results, request further revisions, open a GitHub pull request, or directly integrate the changes". Sources: https://openai.com/index/introducing-codex/ and https://help.openai.com/en/articles/20001545-using-codex-cloud

*Implication:* the harness isolates each task, and a person or a pull request merges. No automated rule decides when a reviewed change needs another review.

**F7.6** The OpenHands documentation index lists pages for sub-agents, for parallel tool calls and for the GitHub resolver. It lists no page about merge conflicts or a landing queue. Source: https://docs.openhands.dev/llms.txt

**F7.7** The Augment guide gives six patterns. They are spec-scoped tasks, one worktree for each agent, a coordinator with specialists and a verifier, model routing, automated gates and sequential merges. It rates a semantic contradiction as "High" in detection difficulty, because it "passes compilation and linting". Source: https://www.augmentcode.com/guides/how-to-run-a-multi-agent-coding-workspace

Tembo's survey puts git worktrees at the center of coding orchestrators. It keeps "a human approval gate so nothing merges unreviewed". Source: https://www.tembo.io/blog/ai-agent-orchestration-tools

*Implication:* the field agrees on isolation, sequential merges and gates. No source here says when an audited change keeps its verdict after its base moves. The tiers fill that gap.

## 8. Prior art: semantic conflicts and test selection

**F8.1** SAM detects semantic merge conflicts with generated unit tests. Over "more than 80 pairs of changes integrated into common class elements from 51 merge scenarios", its best setup found "nine detected conflicts out of 28". Source: https://arxiv.org/abs/2310.02395

*Implication:* tests alone find a minority of semantic conflicts. A real code overlap needs a review as well as the tests.

**F8.2** Ekstazi selects regression tests by the files that each test depends on at run time. Over 615 revisions of 32 projects (almost 5 million lines), it cut the test time by 32% on average, and by 54% for longer suites. Source: https://users.ece.utexas.edu/~gligoric/papers/abstracts/GligoricETAL15Ekstazi.txt

*Implication:* file-level and package-level selection pay off. A test that reads a file outside its own package needs a dependency that the import graph does not show.

**F8.3** Datadog maps each test to the code files that it covers. It skips a test only if a passing run exists for a commit "where the covered and tracked files are identical", and it never skips a test that is marked "unskippable". A change to a tracked file makes all tests run. Source: https://docs.datadoghq.com/intelligent_test_runner/how_it_works

*Implication:* adopt tracked paths (a change runs the full suite) and an unskippable set (the whole-tree tests).

**F8.4** The Go test cache matches a run only for "the same test binary" and cacheable flags. "Tests that open files within the package's module or that consult environment variables only match future runs in which the files and environment variables are unchanged." The idiomatic way to turn the cache off is `-count=1`. Source: https://pkg.go.dev/cmd/go#hdr-Test_packages

The repository runs every test with `-count=1`, because tests read files outside the module root through `..` (CLAUDE.md, Go Project Conventions).

*Implication:* the cache cannot scope our gates. Selection by the import graph does it, and the paths that tests read through `..` are tracked paths.

## 9. Git mechanics

**F9.1** `git merge-tree --write-tree` performs a merge and "does not read from or write to either the working tree or index". Exit 0 means a clean merge, exit 1 means conflicts, and any other exit is an error. "Do NOT interpret an empty Conflicted file info list as a clean merge; check the exit status." With `--merge-base`, "trees are enough". Source: https://git-scm.com/docs/git-merge-tree

**F9.2** A local probe with git 2.50.1 gave these results:

- `git merge-tree --write-tree --merge-base=<base> <peer> <tree>` composes an audited tree directly, with no lane commit.
- In a repository with a worktree, the merge applies `merge=union` from `.gitattributes`.
- In a bare clone, the same merge conflicts on the union file.
- With `git --attr-source=<tip> merge-tree ...`, the bare clone applies the union attribute again.
- A content conflict exits 1, and `--name-only` names the path.

The probe ran in an empty scratch repository:

```sh
git init -q -b main repo && cd repo
printf 'a\nb\n' > reg.txt && printf 'reg.txt merge=union\n' > .gitattributes
git add -A && git commit -qm base && BASE=$(git rev-parse HEAD)
git checkout -qb lane && printf 'a\nb\nlane-line\n' > reg.txt && git commit -qam lane
T0=$(git rev-parse HEAD^{tree}); git checkout -q main
printf 'a\nb\npeer-line\n' > reg.txt && git commit -qam peer && PEER=$(git rev-parse HEAD)
git merge-tree --write-tree --merge-base="$BASE" "$PEER" "$T0"          # exit 0, union applied
cd .. && git clone -q --bare repo bare.git && cd bare.git
git merge-tree --write-tree "$PEER" lane                                 # exit 1, CONFLICT (content)
git --attr-source="$PEER" merge-tree --write-tree "$PEER" lane           # exit 0, union applied
```

*Implication:* the composer always passes `--attr-source=<tip>`. Then the attributes of the tip decide the merge, and the state of a worktree never does. The registry `go/.apicover-enforce` uses `merge=union` (`.gitattributes`, wave 21).

**F9.3** The module import graph lists in 1.8 s with one call: `go list -f '{{.ImportPath}}|{{join .Deps ","}}' ./...` (272 packages at `4f934cf90`).

*Implication:* the overlap proof can compute the closure on each composition. It needs no cache.

## 10. Adopt or reject

| Prior art | Verdict | Reason | Finding |
|---|---|---|---|
| Zuul prefix composition and positional ejection | adopt | Test each change as it will merge; eject the one that fails | F6.1 |
| Zuul speculation for every job | adopt for gates only | An LLM review never runs on a speculative prefix | F6.1, F4.8 |
| GitHub queue: queue-owned composition, removal on red | adopt | The lane never composes itself | F6.2 |
| GitHub queue: full CI for each group | reject | Our costly step is an LLM audit | F6.2, F3.6 |
| Bors serial landing | adopt as the floor | Affordable at our rate when each step is short | F6.3 |
| Uber build-graph independence | adopt for test selection | Run only the packages that both changes affect | F6.4 |
| Uber order change (BLRD) | reject for now | At width 3 the gain is small; it needs every path verified | F6.4 |
| Trunk impacted targets | adopt | The same rule as Uber's, in a product | F6.5 |
| Trunk batch bisection | reject | No batches at width 3 | F6.5 |
| Gerrit `TRIVIAL_REBASE` vote copy | adopt, stricter | T1 also needs disjoint import closures | F6.6 |
| Rust `iffy` and `never` | adopt as the solo slot | Global-zone and protected-surface candidates compose alone | F6.7 |
| TAP presubmit selection, postsubmit backstop | adopt | Scoped gates before the landing; CI on `main` after it | F6.8 |
| Chromium retry with and without the patch | adopt | Separate a flake and a red base from a red candidate | F6.9 |
| Agent teams: file ownership at dispatch | adopt | The dispatch partition | F7.4 |
| Codex and OpenHands: isolate, then a person merges | reject as the whole answer | It has no rule for a verdict after a base move | F7.5, F7.6 |
| Informed agent: show the concurrent change | adopt | The T3 input is the landed diff of the peer | F7.3 |
| Unit-test-only semantic conflict detection | reject as the only check | It found 9 of 28 conflicts | F8.1 |
| Datadog tracked and unskippable tests | adopt | Tracked paths force the full suite | F8.3 |
| Go test cache as the selector | reject | The repository needs `-count=1` | F8.4 |
| `git merge-tree --write-tree` with `--attr-source` | adopt | A merge with no worktree, deterministic attributes | F9.1, F9.2 |
| Package closure from one `go list` call | adopt | 1.8 s for the whole module | F9.3 |
| The file lists of `go list` as the package map | adopt | Embedded files and `testdata/` map to their package; an unowned path is unknown | F4.17 |
| A prose allowlist less the read roots | adopt until a catalog exists | Production code reads three prose roots | F4.18 |
| A registry check on the composed tree | adopt | Two lanes can register one code and each pass alone | F4.20 |
| `PartitionGraph`'s rule (closures meet) | reject | It conflicts 80.5% of pairs | F4.11 |
| Symbol-level overlap (changed names only) | reject | Not sound without a call graph | Synthesis |
| An ML model that predicts success | reject | It needs more history than a fleet of 3 lanes gives | [merge-concurrency-2026](merge-concurrency-2026/README.md) |

## 11. The cost model

The model compares today's measured paths with the expected paths under the landing queue. Each queue step has a source: a measurement (an F-number) or an estimate that the shadow stage measures. Step 4 of §15 gives the arithmetic.

| Queue step | Median (min) | p90 (min) | Source |
|---|---|---|---|
| Compose and prove: `merge-tree`, and `go list` for the four tag sets | 0.2 | 0.3 | estimate; one `go list` takes 1.8 s (F9.3) |
| `compile`: `go vet` for the four tag sets, with a warm cache | 1.0 | 2.0 | estimate |
| `compile` for the `acs` tag set only | 0.3 | 0.5 | estimate |
| `registries`: three checks with the generator of `C` | 0.5 | 1.0 | estimate; a warm `go run` takes under 1 min |
| `test` over `S` | 1.2 | 3.1 | the importer backstop (F3.7) |
| `acs`, `apicover` and `predicates`, scoped | 1.5 | 2.5 | estimate |
| `test` over `A_data(L)` only | 0.3 | 1.0 | estimate |
| Regenerate and check one derived entry | 0.5 | 1.0 | estimate |
| Land: the ship phase on `C` | 2.8 | 5.5 | F3.6 |
| Audit on `C` | 5.7 | 8.3 | F3.6 |
| The interaction review | 5.7 | 8.3 | at most an Audit (F3.6) |
| Build on `C` | 6.3 | 20.0 | F3.6 |

The path of each tier, at the median. Stage 5 of the plan runs T3 with `review: audit`, and stage 6 runs it with `review: interaction`:

| Tier | Path | Stage 5: min, LLM phases | Stage 6: min, LLM phases |
|---|---|---|---|
| T1, by a bookkeeping-only peer delta | compose, `test` over `A_data(L)`, land; `compile` for the `acs` tag set when the peer delta holds predicates | 3.3, 0; 3.6 with the compile | 3.3, 0; 3.6 with the compile |
| T1, disjoint | compose, the gates, land | 7.2, 0 | 7.2, 0 |
| T2 | as T1, with a regeneration | 7.7, 0 | 7.7, 0 |
| T3, no shared path, stage 5 | compose, eject with `needs_audit`, Audit on `C`, compose again with a bookkeeping-only peer delta, land | 9.2, 1 | none |
| T3, no shared path, stage 6 | compose, review, the gates, land | none | 12.9, 1 |
| T3, a shared path | compose, eject with `needs_build`, Build and Audit on `C`, compose again, land | 15.5, 2 | 15.5, 2 |
| T4 | today's debugger route (F3.2) | 25.8, 2.33 | 25.8, 2.33 |

Two of these paths rest on an estimate of the order of events:

- The stage 5 path of T3 assumes that no peer lands during the Audit on `C`. Then its second composition has a bookkeeping-only peer delta. A peer that lands in that window adds a composition and the gates of its tier.
- The bookkeeping path adds the `acs` compile only when the peer delta holds predicates under `go/acs/`. That was 1 of the 7 cycles: 1832.

Today against the queue, for the 39 classified cycles (F3.9):

| Class | Cycles | Today: mean (min) | Today: median (min) | Today: LLM phases | Stage 5: min, LLM phases | Stage 6: min, LLM phases |
|---|---|---|---|---|---|---|
| T1, by a bookkeeping-only peer delta | 7 | 17.6 | 12.9 | 1.43 | 3.3, 0; 3.6 for 1832 | 3.3, 0; 3.6 for 1832 |
| T1, disjoint | 11 | 20.5 | 18.4 | 1.45 | 7.2, 0 | 7.2, 0 |
| T2 | 0 | none measured | none measured | none measured | 7.7, 0 | 7.7, 0 |
| T3, no shared path | 17 | 25.8 | 27.9 | 1.24 | 9.2, 1 | 12.9, 1 |
| T3, a shared path | 2 | 29.4 | 29.4 | 2.00 | 15.5, 2 | 15.5, 2 |
| T4 | 2 | 45.0 | 45.0 | 4.00 | 25.8, 2.33 | 25.8, 2.33 |
| All | 39 | 24.0 | 22.1 | 1.51 | mean 8.8, median 9.2; 0.66 | mean 10.4, median 12.9; 0.66 |

- The cost of a cycle today is the sum of its recoveries. Cycle 1719 had two.
- Today's route does not follow the tier. A disjoint rebase paid 20.5 min and 1.45 LLM phases, close to a coupled one.
- Stage 5 costs 63% fewer minutes and 56% fewer LLM phases than the same cycles today. The median falls from 22.1 to 9.2 min.
- The median of each stage is the cost of T3 with no shared path, because that class holds the 20th of the 39 values.
- Stage 6 costs more minutes than stage 5, with the same LLM phases. Its T3 runs a review and the gates, and stage 5 runs one Audit on `C` in their place. Stage 6 pays only when a review costs fewer tokens than an Audit, or finds interactions that an Audit misses. The plan opens stage 6 on a measured token saving.
- Head-of-line wait is a new cost. I replayed the first ship start of each cycle from 1691 to 1843 (107 starts) through one FIFO queue (§15, step 5). With a step of 10.4 min, 9% of the ships waited. The mean wait was 0.53 min, the p90 was 0 and the maximum was 8.3 min.
- With a step of 15.5 min, 12% of the ships waited. The mean wait was 1.14 min, the p90 was 4.9 min and the maximum was 13.4 min.
- The step is the whole path of a candidate, an upper bound. A candidate composes and runs its gates before it reaches the head.
- The dispatch partition (B2) moves up to 4 of the 39 pairs out of one wave (F3.10). The model does not count that gain.

## 12. The approaches

The forces that decide between the approaches:

- **Cost.** LLM minutes and phases for each rebase (F3.2).
- **Safety.** No change lands without a check that fits its overlap with its peers. Unknown overlap is never treated as disjoint.
- **Liveness.** No lock across LLM work (F4.8); a red candidate never blocks the green ones.
- **Throughput.** Lanes keep running in parallel; landings can be serial (F6.3).
- **Determinism.** Deterministic work is code; only judgment is an LLM phase (AGENTS.md rule 5).
- **Conservation of proven work.** A verdict that still holds is kept ([ADR-0048](../architecture/adr/0048-work-conservation-fast-reland-resilient-ship.md)).

### 12.1 A: a landing queue with tiered re-verification (the winner)

An audited lane does not merge itself. It enters one landing queue, which composes it on top of the candidates ahead. An overlap proof over the peer delta, the lane's change and both import closures picks the check. T1 runs gates only, T2 regenerates and runs gates, T3 adds an interaction review, and T4 ejects. The [spec](../architecture/fleet-landing-queue.md) holds the rules.

### 12.2 B: avoidance at dispatch, alone

Partition the todos of a wave by import closure and the global zone, and regenerate the derived projections at the rebase. Keep today's recovery ladder for what remains.

**Steelman.** Prevention costs less than repair. Uber, Trunk and TAP all decide independence from the build graph (F6.4, F6.5, F6.8). Claude Code agent teams and the Augment guide both say that each agent owns its own files (F7.4, F7.7). AgenticFlict and the study of 33,596 pull requests both show that overlap drives conflicts (F7.1, F7.2). B needs no new landing machinery, and it removes the overlaps that it can see before any LLM runs.

**Rejected alone:**
- Each lane after the first still diverges, because `main` moved (F4.7). B gives no rule for the verdict after that rebase, so today's 24.2-min ladder still runs on every rebase.
- Declared footprints are not the real diffs: a lane can touch more than its card declares.
- The closure rule of `PartitionGraph` conflicts 80.5% of package pairs (F4.11), so B at full strength collapses fleet width.
- Its two cheap parts are adopted: the derived catalog (F2.7) and the dispatch partition with a narrower rule (F4.11).

### 12.3 C: stacked speculation

Start lane N+1 on top of the branch of lane N, so that lanes compose at build time and never rebase at landing.

**Steelman.** Each lane sees its peers' code while it builds, so the "informed agent" effect applies (F7.3: 82% recovered). Nothing rebases at the end, and Zuul and SubmitQueue both speculate (F6.1, F6.4). Stacked branches are normal practice for human teams.

**Rejected:**
- A failure cascades. 6 of 45 recoveries did not ship (F3.1). Waves 82 and 83 shipped 1 of 3 and 1 of 2 cycles (the goal of wave 84). Each lane stacked on a failed lane must rebuild, and an LLM Build takes 6.3 min at the median and 20 min at p90 (F3.6).
- The lanes stop being independent, so one slow lane holds the lanes above it.
- Zuul and SubmitQueue speculate with cheap CI jobs, not with LLM builds that run for 6 to 20 minutes (F3.6).

### 12.4 Fleet width 1

Run one lane at a time, so that no lane ever diverges.

**Steelman.** It removes the whole problem with no new code. The scale ladder of the earlier research says that plain serialization is affordable below 10 changes a day ([merge-concurrency-2026](merge-concurrency-2026/README.md)). Our rate is about 4 cycle landings a day (F3.8).

**Rejected:**
- Bors serializes the landing, which takes minutes. Width 1 serializes the work, and a cycle takes an hour or more.
- `fleet.count=3` was restored on purpose (CLAUDE.md). Width 1 cuts throughput to about a third.
- The cost of a rebase at width 3 (24.2 min, F3.2) is smaller than two lost lane slots.

### 12.5 E: an external merge queue

Lanes open pull requests, and the GitHub merge queue (or Mergify, Trunk or Graphite) lands them.

**Steelman.** It is a mature product with no queue code to own. It has the composition, the removal on red and the concurrency cap (F6.2). It runs the CI of the repository, so `main` stays green by construction.

**Rejected:**
- It knows nothing about an audit verdict, so a red check sends the lane back with no tier: the queue has no T1 to T3.
- Each group runs the full CI on GitHub runners: about 10 to 15 min of tests for each candidate, plus the queue latency.
- Lanes land on `main` through `evolve ship` and its audit binding (ADR-0039 §8.1). A pull request path is a new main-push route around those checks.

### 12.6 Scores

| Force | A: queue and tiers | B alone | C: stacked | Width 1 | E: external queue |
|---|---|---|---|---|---|
| Minutes for each rebase | 8.8 expected at stage 5 (§11) | 24.2 (ladder stays) | 0 at landing; cascades on failure | none | 10 to 15 of CI, plus a full re-audit on red |
| LLM phases for each rebase | 0.66 expected | 1.49 | cascade rebuilds | 0 | 1.49 or more |
| Safety on coupled changes | T3 review and scoped gates | today's ladder | build-time visibility | trivial | CI only |
| Liveness | no lock across LLM work | as today | one slow lane holds the stack | trivial | depends on CI |
| Throughput | 3 lanes; serial landing | 3 lanes, fewer co-dispatched | 3 lanes, coupled | 1 lane | 3 lanes |
| New code | the queue, the proof, the review phase | small | a stack manager | none | a pull request path and webhooks |
| Fit with the audit binding | reuses the carry record and the ship acceptance | as today | new | as today | a second route to `main` |

## 13. The winner

Approach A with two cheap parts of B, as the operator decided:

- **A.** Finish ADR-0078 as a durable landing queue. The queue decides the order and the composition, and the head lane lands. An overlap proof picks T1, T2, T3 or T4, and unknown overlap is never T1.
- **B1.** The skill and command projections join one derived catalog, so a rebase regenerates them (F2.7, F4.14).
- **B2.** `PartitionGraph` joins dispatch with a narrower rule, and it falls back safely for a todo with no files (F4.10, F4.11).

Two details differ from the literal brief:

- The dispatch rule is "same package, same file or the global zone", not "closures meet". The closure rule conflicts 80.5% of package pairs (F4.11). Cross-package overlaps go to T3 at landing. The operator confirmed this rule on 2026-10-09 ([plan](../plans/concurrent-cycle-landing-2026-10.md) O10).
- The queue decides, and the head lane lands its own composed commit. A separate composer process that lands for other cycles needs the ship state of each cycle in one process. Each lane is its own process (F5.3). This is a console decision that the operator can override (plan CD1).

The operator also decided three details of the spec on 2026-10-09:

- package edges count in both directions (O11);
- a red tip pauses the queue as a system failure (O12);
- the review runs on Opus 5.5 at medium effort (O13).

## 14. Refinements

1. **R1** Compose the audited tree, never ship's commit; ship's inbox consumption comes at the landing. [F2.2, F9.2]
2. **R2** Report "the proof did not run" apart from "the proof failed". [F2.2]
3. **R3** Fix the continuation decline in today's ladder until the queue enforces. [F2.4]
4. **R4** One derived catalog with whole-file and region outputs, a generator and a check. [F2.7, F4.14]
5. **R5** Regenerate with the composed tree's own generator, never the host binary's. [F2.6]
6. **R6** After a regeneration, refuse a file that still holds conflict markers. [F9.1]
7. **R7** The overlap proof uses the import closure in both directions, at the composed tree. [F3.9, F6.4]
8. **R8** A shared leaf import is not an overlap; a directed package edge is. [F4.11]
9. **R9** Unknown overlap goes to T3, never to T1. [operator decision]
10. **R10** Bookkeeping paths are outside the proof and outside test impact. [F3.8]
11. **R11** Scope the tests to `(A(L) ∩ A(P)) ∪ A_data(L)`: the tests that both changes reach, and the tests that read the lane's paths. Ship's fixed pack is the unskippable set. [F6.4, F8.3, F4.13]
12. **R12** A tracked path, or a closure that cannot be listed, selects the full suite. [F8.3, F8.4]
13. **R13** Screen a red: rerun with the candidate, then run without it. [F6.9]
14. **R14** A red without the candidate pauses the queue as a system failure. [F6.9, operator decision]
15. **R15** The T3 input is the landed diff of the peer and the lane's change. [F7.3]
16. **R16** The T3 review judges interactions only; it never re-audits the lane. [F7.3, F8.1]
17. **R17** No LLM review runs on a speculative prefix. [F4.8, F6.1]
18. **R18** Positional ejection: the candidate that fails leaves; the green prefix lands. [F6.1, F6.2]
19. **R19** Every ejection has a repair route. [F5.3]
20. **R20** The queue lives on disk, under the protected `.evolve/landing/` tree. [F5.3]
21. **R21** Compose with `merge-tree --write-tree` and `--attr-source=<tip>`. [F9.1, F9.2]
22. **R22** An `iffy` candidate gets a solo slot. [F6.7]
23. **R23** Dispatch keeps one package, one file and the global zone in one lane. [F3.10, F4.11]
24. **R24** A todo with no files keeps today's file partition; it never fails a wave. [F4.10]
25. **R25** Re-run the lane's own predicates on the composed tree, with a fresh receipt. [F4.16]
26. **R26** Map each path under `go/` through the file lists of `go list` and the `testdata/` trees. An unowned path is unknown. [F4.17]
27. **R27** Put `go/vendor/` and each `.gitattributes` and `.gitignore` in the build zone. [F4.17]
28. **R28** Trust only the prose outside the read roots. Each other path is unknown until a catalog of production reads exists. [F4.18]
29. **R29** Compile the composed tree with its tests under each tag set of the `Makefile`. [F4.19]
30. **R30** Run the registry checks of the composed tree with its own generators. A code that two packages register is red. [F4.20]
31. **R31** Bound the review with a deadline. Past it, the verdict is `UNSURE`, and the window keeps its size. [F4.8]
32. **R32** An owner takes its lock before its record is visible. A waiter releases the lock that woke it. The landing intent decides a dead owner in `landing`. [F4.8, F5.3]
33. **R33** Measure escapes in shadow: run the CI suite on the composed tree, with its tip as the control. [F6.8, F6.9]

## 15. Reproduction

A throwaway Go program in the lane scratchpad made the tables. It reads the runtime runs and git, and it changes nothing. Plan component Q18 makes it a product verb, `evolve landing census`. These are its steps:

1. **The recovery census** (F3.1, F3.2). Read each `runtime/.evolve/runs/cycle-*/phase-timing.json`.
   - A recovery starts at the end of each `ship` row whose `abort_reason` holds `GIT_FLEET_REBASE_NEEDED`. The route is the word after "recovering via".
   - It ends at the end of the next `ship` row with the verdict `PASS`. Without such a row before the next rebase error, it ends with the last recovery row.
   - A recovery row is a Build, Audit, debugger, TDD or ship row. The LLM phases are the Build, Audit, debugger and TDD rows of the recovery.
2. **The package map** (F4.17, F9.3). In `go/`, run `go list` once for each tag set: none, `integration`, `acs`, and `e2e evolve_test_phases` (the command below). The map is the union of the four outputs. Each tracked path under `go/` then gets its class by the zones of the [spec](../architecture/fleet-landing-queue.md) §6.
3. **The tier census** (F3.9). For each cycle with a `ship-binding.json`, `L` is `git diff --name-only --no-renames <landing>^ <landing>`. `P` is `git diff --name-only --no-renames <pre_cycle_head> <landing>^`, with `pre_cycle_head` from `run.json`. Apply the zones of spec §6 and the rules of spec §8, with the map of step 2. Cycles 1719 and 1792 had a genuine conflict (the debugger route).
4. **The cost model** (§11). The cost of a tier path is the sum of its step costs. The cost of a cycle today is the sum of its recoveries from step 1.
5. **The queue-wait replay** (§11). Take the start of the first `ship` row of each cycle from 1691 to 1843, and sort them. Serve them in order, with one server and a fixed step. A wait is the start of the service less the arrival.
6. **The graph statistics** (F4.11). Count the pairs of module packages outside `/acs/` that import another module package, with the default tags. Two closures meet when `(deps(a) ∪ {a}) ∩ (deps(b) ∪ {b})` is not empty. The directed rule holds when `a ∈ deps(b) ∪ {b}` or `b ∈ deps(a) ∪ {a}`.

The `go list` call of step 2, for the tag set `integration`:

```sh
cd go && go list -tags integration -f '{{.ImportPath}}|{{.Dir}}|{{join .Deps ","}}|{{join .GoFiles ","}},{{join .CgoFiles ","}},{{join .IgnoredGoFiles ","}},{{join .CFiles ","}},{{join .CXXFiles ","}},{{join .MFiles ","}},{{join .HFiles ","}},{{join .FFiles ","}},{{join .SFiles ","}},{{join .SwigFiles ","}},{{join .SwigCXXFiles ","}},{{join .SysoFiles ","}},{{join .IgnoredOtherFiles ","}},{{join .EmbedFiles ","}},{{join .TestGoFiles ","}},{{join .XTestGoFiles ","}},{{join .TestEmbedFiles ","}},{{join .XTestEmbedFiles ","}}' ./...
```
