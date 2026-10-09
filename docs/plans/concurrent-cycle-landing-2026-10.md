# Plan: concurrent cycle landing, a landing queue with tiered re-verification

- **Status:** design, opened 2026-10-09 in the docs lane `cl-landing-queue` (branch `docs/landing-queue-tiered-reverify`, base `4f934cf90`). The operator chose the approach on 2026-10-09. This file holds the decisions, the components, the migration and the rollout.
- **Spec of record:** [fleet-landing-queue.md](../architecture/fleet-landing-queue.md). It holds every rule. This plan links it and does not restate it.
- **Decision record:** [ADR-0128](../architecture/adr/0128-landing-queue-tiered-reverification.md).
- **Research:** [concurrent-cycle-landing-2026-10.md](../research/concurrent-cycle-landing-2026-10.md) (findings F2.1 to F9.3, refinements R1 to R33).

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Operator decisions](#4-operator-decisions)
5. [Candidate designs](#5-candidate-designs)
6. [Decisions](#6-decisions)
7. [Components](#7-components)
8. [TDD protocol](#8-tdd-protocol)
9. [Migration from the current ladder](#9-migration-from-the-current-ladder)
10. [Rollout](#10-rollout)
11. [Metrics](#11-metrics)
12. [Verification](#12-verification)
13. [Patterns and forces](#13-patterns-and-forces)
14. [Risks](#14-risks)
15. [Open questions](#15-open-questions)
16. [Status](#16-status)

## 1. Request

The operator wrote this on 2026-10-09:

1. *"Design a better solution to improve the situation: rebase + re-audit every time one cycle is done and another completes later. Consider multiple cycles running at the same time and conflict. Research how other agent/harness solutions solve this. Propose at least 2 approaches and pick the winner."*
2. *"all docs need to be properly written in detail under docs/"*

The operator also chose the winner and named the parts of it (§4).

## 2. Facts

The research holds the evidence. These facts drive the plan:

| Fact | Evidence |
|---|---|
| A fleet-rebase recovery costs 24.2 min and 1.49 LLM phases on average; 45 recoveries cost 1,091 min in 14 days. | research F3.2 |
| In cycle 1843, the identity proof did not run, because the B1 unwind declines for a continuation lane. The log said "not proven identical". | F2.1, F2.2 |
| Cycles 1801, 1818 and 1843 are one class: 102.8 min and 7 LLM phases since 2026-10-06. | F2.4 |
| Cycle 1841 spent 61.6 min and did not ship: a full-suite `apicover` red, then a false red of the drift gate. | F2.5, F2.6 |
| The full composed gates take 11.3 min. They declined 10 carries, and 8 of those lanes shipped the same tree after one Audit. | F3.4, F3.5 |
| The ship's importer backstop runs a scoped test set in 74 s at the median. | F3.7 |
| Of 39 classified rebases, 18 are T1 (7 with a bookkeeping-only peer delta), 19 are T3 (2 with a shared path) and 2 are T4. | F3.9 |
| The prefix queue of ADR-0078 has no production caller. | F5.1, F5.2 |
| The rule of `PartitionGraph` conflicts 80.5% of package pairs. | F4.11 |
| Today's carry does not re-run the lane's own predicates on the composed tree. | F4.16 |
| Dossier closeouts are the most frequent writer of `main`: 53 of 119 commits since 2026-10-01. | F3.8 |
| `go list` names 21 embedded files as compile inputs, 210 tracked files sit in `testdata/` trees, and 12 tracked paths under `go/` have no package. | F4.17 |
| Production code reads three prose roots: the STE standard, `docs/research/` and `docs/reference/`. | F4.18 |
| No check runs on the composed tree before a landing. | F4.19 |
| A code that two packages register is a recorded conflict, not a build error. | F4.20 |

## 3. Goals and non-goals

Goals:

- **G1.** A rebase with no real overlap lands with no LLM phase (T1).
- **G2.** A rebase with a real code overlap gets one interaction review, not a Build and a full Audit (T3).
- **G3.** Unknown overlap is never treated as disjoint.
- **G4.** The composed gates run in minutes: scoped by test impact, with a flake screen.
- **G5.** No lock is held across LLM work, and a red candidate never blocks the green candidates.
- **G6.** Each continuation lane reaches the identity check.
- **G7.** The derived projections have one home, and a rebase regenerates them.
- **G8.** Dispatch keeps compilation units apart without a collapse of fleet width.
- **G9.** Each step is observable: signals, a shadow record and the metrics of §11.

The spec holds the non-goals ([fleet-landing-queue.md](../architecture/fleet-landing-queue.md) §19 and Limits). This plan adds three:

- no feature flag and no env flag, because each dial is a config key;
- no change for width 1, for `per-lane` or for the two-phase landing;
- no new dependency in `go/go.mod`.

## 4. Operator decisions

| # | Decision | Source |
|---|---|---|
| O1 | The winner is Approach A: finish and turn on the ADR-0078 `fleet.landing=prefix-queue`, Zuul-style. | operator, 2026-10-09 |
| O2 | An audited lane does not merge itself. It enters one landing queue, which composes it on top of the candidates ahead. | operator |
| O3 | The tiers are T1 disjoint, T2 derived only, T3 a real code overlap and T4 a conflict or a red gate. | operator |
| O4 | T3 is an interaction-only LLM re-audit of the peer's landed diff and the lane's diff, with no full Build and no full Audit. | operator |
| O5 | Unknown or uncertain overlap is never T1; it goes to T3. | operator |
| O6 | The first cheap part of B: the skill and command projections join `derivedArtifacts`, so the rebase regenerates them. | operator |
| O7 | The second cheap part of B: wire `PartitionGraph` into dispatch, with a safe fallback for todos that carry only `[id]`. | operator |
| O8 | Rejected: B alone, C (stacked speculation) and fleet width 1. | operator |
| O9 | All docs are written in detail under `docs/`. | operator |
| O10 | The dispatch partition conflicts two todos on the same file, the same package or the global zone. The default is `package`; `closure` stays an option. | operator, 2026-10-09 (was OQ1) |
| O11 | Package edges count in both directions. | operator, 2026-10-09 (was OQ3) |
| O12 | A red composed tip pauses the queue as a system failure under ADR-0072, and the loop halts. | operator, 2026-10-09 (was OQ4) |
| O13 | The T3 review runs on Opus 5.5 at medium effort. The effort rises only after measured misses. | operator, 2026-10-09 (was OQ6) |

The console decided the other four open questions on their recommendations. The operator can override each one:

| # | Decision | Source |
|---|---|---|
| CD1 | The head lane lands its own composed commit. No separate composer process lands for other cycles. | console, 2026-10-09 (was OQ2) |
| CD2 | The other writers of `main` (dossiers, inbox stamps) stay outside the queue. Shadow measures their compositions. | console (was OQ5) |
| CD3 | The B1 to B5 ladder retires for fleets at stage 7, after the exit criteria hold. `per-lane` keeps only the full re-audit route. | console (was OQ7) |
| CD4 | T3 with a shared path keeps its Build. The plan measures the share first, and extends the rebind only if the share grows. | console (was OQ8) |

## 5. Candidate designs

The research compares five approaches with forces, steelmen and scores ([research](../research/concurrent-cycle-landing-2026-10.md) §12):

| Approach | Verdict | The main reason |
|---|---|---|
| A: a landing queue with tiered re-verification | the winner (O1) | 8.8 min and 0.66 LLM phases expected for each rebase at stage 5 |
| B alone: avoidance at dispatch | rejected (O8); two parts adopted | it gives no verdict rule after the rebase |
| C: stacked speculation | rejected (O8) | a failure cascades into LLM rebuilds |
| Fleet width 1 | rejected (O8) | it serializes the work, not only the landing |
| E: an external merge queue | rejected | it knows no audit verdict, and it opens a second route to `main` |

## 6. Decisions

| # | Decision | Reason |
|---|---|---|
| D1 | The queue store is a set of files under `.evolve/landing/queue/`, with one record for each candidate (spec §2). | Each lane is its own process, and the tree is protected (F5.3). |
| D2 | The queue decides, and the head lands. Each owner works its own candidate, and no process runs the ship of another cycle (spec §1). | ADR-0078 named one composer process. That process needs the ship state of every cycle (CD1). |
| D3 | The order is FIFO by ticket, with no order change (spec §4). | An order change needs every path of the speculation tree verified (F6.4). |
| D4 | Compose the audited tree with `merge-tree --write-tree`, `--attr-source=<tip>` and `--merge-base=<base0>` (spec §5). | It removes the class of cycle 1843, and the attributes are deterministic (F2.2, F9.2). |
| D5 | The proof counts package edges in both directions (spec §6). | A lane change inside the peer's closure can break the peer's code (O11; F3.9). |
| D6 | The tier uses production imports. The test selection uses test imports too (spec §6, §9). | A test helper change is a test risk, not a semantic overlap. |
| D7 | The strictest rule wins, and unknown is T3 (spec §8). | O5 |
| D8 | T3 with a shared path ejects with `needs_build` (spec §8, §11). | The explanation rebind needs byte identity of the lane's paths; 2 of 39 pairs (F3.9). |
| D9 | T3 is the phase `landing-review` on the auditor's seat: Opus 5.5 at medium effort. In `enforce`, `review` starts at `audit`, then moves to `interaction` (spec §10, §14). | O4, O13 |
| D10 | No review runs on a prefix that has not landed. A T3 candidate reviews only at the head (spec §4). | No LLM work is wasted when a candidate ahead ejects (R17). |
| D11 | The selection is `(A(L) ∩ A(P)) ∪ A_data(L)`, and it reuses `changedpkgs.ImporterClosureChecked` and `regressiontia` (spec §9). | F4.12, F6.4, R11; no floor runs the lane's data readers before CI (F4.19). |
| D12 | The gate set is `compile`, `registries`, `test`, `acs`, `apicover` and `predicates`, with a fresh receipt on `C` (spec §9). | F4.16, F4.19, F4.20, R25, R29, R30 |
| D13 | A red reruns alone, then runs on the tip. A red tip pauses the queue, and the loop halts (spec §9). | F6.9, O12 |
| D14 | T1 reuses the record method `identical-rebase`. T2 and T3 add `derived-regen` and `interaction-review`, and ship proves each claim again (spec §11). | The B4 rule of ADR-0105. |
| D15 | For T2, the explanation rebind drops the derived outputs from its identity domain (spec §11). | F4.15 |
| D16 | Each ejection reason has one route, through new ship errors and router rules (spec §12, §15). | Every ejection has a repair route (R19). |
| D17 | The window keeps the AIMD rule of ADR-0078, and an `iffy` candidate gets a solo slot (spec §4). | F6.1, F6.7 |
| D18 | The budgets of spec §13 apply, and each ejection costs one unit of `shipRecoveryBudget`. | A bounded recovery, as today (F4.3). |
| D19 | One derived catalog, `internal/derived`, serves the normalizer, the rebase, the proof and the checks (spec §7.1). | O6, R4 |
| D20 | A regeneration uses the generator of the composed tree and refuses a leftover conflict marker (spec §7.1). | F2.6, R5, R6 |
| D21 | Dispatch uses `PartitionGraph` with the relation `package` by default, and `closure` as the option (spec §17). | O7 with a narrower relation: the current rule conflicts 80.5% of pairs (O10; F4.11). |
| D22 | A todo with no files keeps today's file partition, and a `go list` failure falls back to it (spec §17). | O7 (R24) |
| D23 | The dials are `fleet.landing`, `fleet.landing_queue.*` and `fleet.partition.*`. Unknown values fail safe (spec §16). | Config, not flags (the house rule). |
| D24 | Shadow computes the queue's decision at each fleet rebase, with no LLM phase and no gate. Its escape oracle runs at the wave boundary (spec §14). | The metrics of §11 before any change of behavior (R33). |
| D25 | Before `enforce`, today's ladder runs the identity proof for continuation lanes and reports a skipped proof (component Q12). | F2.2, F2.4; the queue needs weeks to enforce. |
| D26 | At the default flip, the paths of §9 marked "retire" go away. `per-lane` keeps the full re-audit route. | One ladder to maintain (CD3). |
| D27 | The operator verbs are `evolve landing queue status`, `resume`, `eject` and `shadow-verify` (spec §16). | Each control goes through the `evolve` CLI. |
| D28 | The codes are `SHIP_QUEUE_*` in the module `ship`, with the new kind `ship.queue` (spec §15). | The prefix rule of the Signal Center. |
| D29 | The catalogs are code with guard tests, not config (spec §7). | A config list drifts from the code that it describes. |
| D30 | This lane adds one status sentence to four docs that call the prefix queue live. | F5.4 |
| D31 | The global zone has a build zone and a gate zone. The build zone gains `go/vendor/**` and each `.gitattributes` and `.gitignore`. The gate zone holds `go/Makefile`, `go/.cover-strict`, `go/.apicover-enforce` and the CI workflows. All its paths become relative to the repository root (spec §7.3). | Card files are relative to the root, and today's list mixes the two bases. A vendored file is a build input (F4.17). A gate change needs the full suite, not a review. |
| D32 | A waiting candidate blocks in `flock` on the owner lock of the candidate just ahead. It releases that lock at once when `flock` returns (spec §3). | ADR-0127 forbids a poll, and `flock` needs no new package. A waiter that kept the lock can block the queue. |
| D33 | After the enforce soak meets the exit criteria of §10, `prefix-queue` with `enforce` becomes the compiled default for fleets. | O1; ADR-0078 kept `per-lane` only because the queue was not soaked. |
| D34 | A peer delta of bookkeeping paths only runs one gate, `test` over `A_data(L)`. Gate results carry across a composition that adds only bookkeeping (spec §9). | Dossier closeouts are 53 of 119 commits on `main` (F3.8); without the carry, a head runs its gates again for each. |
| D35 | In `enforce`, a queue lane takes no ship-window lease (spec §14). | The queue orders the landings; the lease holds lanes back before they enqueue (F4.7). |
| D36 | The package map holds the file lists of `go list` over the four tag sets. A path under `go/` that no package owns is unknown (spec §6). | Embedded files and `testdata/` files are inputs of their package (F4.17, R26). |
| D37 | Prose is Markdown under `docs/` and `solutions/`, outside three read roots. Each other path is unknown until a catalog of production reads exists (spec §6). | Production code reads the read roots (F4.18, R28). |
| D38 | The review has the deadline `review_timeout_minutes`. Past it, the verdict is `UNSURE`, and the window keeps its size (spec §10). | A slow review holds the head (ADR-0049 S5b, R31). |
| D39 | The owner takes its owner lock before its record is visible. A waiter wakes only when the candidate just ahead is terminal or dead (spec §3). | No client reads a live owner as dead. At width 3, a wake on each change of a composed ref gains little (research §11, R32). |
| D40 | For a dead owner in `landing`, the landing intent decides. No intent or an `unwound` intent ejects, `prepared` pauses, and `complete` gives `landed` (spec §3). | Only a `prepared` intent can leave `main` half moved (ADR-0039 §8.1, R32). |
| D41 | T1 re-runs on `C` each check whose inputs both changes reach. e2e, `cover-strict` and the integration-tagged importer tests outside `S` stay after the landing (spec §9). | No check runs on `C` today (F4.19). |
| D42 | The `compile` gate vets the four tag sets with the test files (spec §9). | A tagged file compiles only under its tag (R29). |
| D43 | The `registries` gate runs the three checks with the generator of `C`, and Q11 makes the codes check fail on a conflict (spec §9). | F4.20, R30 |
| D44 | Shadow keeps each T1 and T2 tree. At the wave boundary, `shadow-verify` runs the gate set and the CI suite on it, and the CI suite on its tip (spec §14). | The escape metric needs an oracle on `C` (R33). |
| D45 | The census, the cost model and the wait replay become the Go verb `evolve landing census` (Q18). | All tooling is Go, and the tables of the dossier stay reproducible. |
| D46 | The `apicover` gate passes `APICOVER_PKGS` as a command-line variable of `make`, not in the process environment (spec §9). | The rule of command-line parameters over env flags. `make` exports a command-line variable to its recipes. |
| D47 | In a paused queue, each owner parks. Its record becomes `parked` with its ticket and evidence, and the owner releases its lock. Its ship ends with `LANDING_QUEUE_PAUSED`, which a floor makes a system failure, so the loop halts. After `resume`, the cycle resume re-attaches each candidate (spec §3). | No process waits through a pause, so nothing polls (ADR-0127). A wait on a lock that `resume` releases needs a holder that lives through the halt. The death of that holder wakes each waiter into a pause that still holds. A halted loop keeps no lanes alive (ADR-0072). |
| D48 | Only a candidate whose tip is the `main` tip pauses the queue on a red. A red tip that holds a candidate that has not landed keeps the candidate waiting. A `compile` red runs on the tip too (spec §9). | A red on a prefix that has not landed is not a red `main`, so it is no system failure. A compile red that the `main` tip shares must not make an innocent candidate T4. |
| D49 | A bookkeeping-only peer delta that holds predicates under `go/acs/` also runs `compile` for the `acs` tag set (spec §8, §9). | The predicates are Go code that only that tag compiles, and a lane change can break them (research F3.9: cycle 1832). Out of bookkeeping, they add a package edge each time the lane changes a package that they import. |
| D53 | The interim fix (Q12) routes a continuation lane to Audit, not to Ship, when its rebase is byte-identical. The inbox item [`ship-carry-accepts-a-kept-consumption`](../../.evolve/inbox/2026-10-09T12-00-00Z-ship-carry-accepts-a-kept-consumption.json) holds the Ship route; Q14 removes the class. That item also owns the check that a re-ship is idempotent when `consumed/<name>` is already present. | The lane keeps ship's consumption in its change. Ship's carry check (B4) re-proves the bytes and does not accept that consumption, so a carry record gives a false `INTEGRITY_TREE_DRIFT` (research F2.3). |

## 7. Components

Each component is small, and it lands unwired first. The wiring is a separate commit with its integration tests. Each new package is at 100 in `go/.cover-strict` and listed in `go/.apicover-enforce`. Each changed function in a mixed package reaches 100% line coverage.

| # | Component | Reuses | Protected surface |
|---|---|---|---|
| Q0 | Docs: this plan, the spec, ADR-0128, the dossier, the indexes and the status sentences (this lane) | the sibling docs | no |
| Q1 | `internal/derived`: the catalog (O6), then the rewire of `normalizeDerivedProjections` and `rebaseWithDerivedRegen` | `skillcheck` markers, `WorktreeEvolveInvocation` | yes (`core` recovery) |
| Q2 | The dispatch partition (O7): the narrower relation, repository-relative paths, the global zone, the config resolver and the shadow wiring | `PartitionGraph`, `PlanFromTriage` | no |
| Q3 | `internal/overlap`: the overlap proof, a pure function | `explanationdocs.isPlainPath`, exported for reuse | no |
| Q4 | `landing.Compose`: `merge-tree`, the conflict classes and the materialization | `landing.Landing`, `pendRebasedChange` | yes (ship landing) |
| Q5 | `internal/testimpact`: the selection, the full-suite triggers and the data-edge catalog with its scan | `changedpkgs`, `regressiontia` | no |
| Q6 | `internal/landingqueue`: the store, tickets, the lifecycle, the window, owner locks, budgets, the pause and parking | `atomicwrite`, `adapters/flock` | yes (new, added to the manifest) |
| Q7 | `ship.ComposedGates`: the gate set and the flake screen with the run on the tip | the ship pack runner, `acssuite` receipts | yes (ship) |
| Q8 | Records and ship acceptance: the two methods, the scope fields and the receipt on `C` | `carrySatisfied`, the ledger composition records | yes (ship, ledger) |
| Q9 | The explanation rebind for T2 | `RebindIdenticalRebase` | yes (`explanationdocs`) |
| Q10 | The `landing-review` phase: registry entry, persona, skill, routing, prompt builder and report checks | `evolve phases create`, the auditor's seat | yes (registry, routing) |
| Q11 | Signals, ship errors and router rules; the codes check fails on a registry conflict; the floor that halts the loop on `LANDING_QUEUE_PAUSED` | `signalcenter.RegisterCode`, `router.recoveryChain`, `core.SystemFailureSignal` | yes (router, `core`) |
| Q12 | The interim fix in today's ladder: the skipped-proof report, and the proof for continuation lanes | `routeRebasedExplanation`, `consumptionDecline` | yes (`core` recovery, `explanationdocs`) |
| Q13 | The shadow wiring in `recoverFromShipError`, with the kept trees, and the escape oracle in the boundary of `evolve wave next` | Q3 to Q7 | yes (`core` recovery) |
| Q14 | The enforce wiring in the ship phase: enqueue, compose, prove, gate and land | Q3 to Q9, Q11 | yes (ship, `core`) |
| Q15 | The waits: the owner-lock protocol, the kernel wait and the liveness check | Q6, `adapters/flock` | yes (ship) |
| Q16 | The orchestrator edges `ship → landing-review → ship`, after Q10 | Q10, the phase registry | yes (registry, `core`) |
| Q17 | The verbs `evolve landing queue status`, `resume`, `eject` and `shadow-verify` | Q6, Q13 | no |
| Q18 | The verb `evolve landing census`: the recovery census, the tier census, the cost model and the wait replay of research §15 | Q3, `changedpkgs` | no |
| Q19 | The retirement at the default flip (§9) | none | yes |

- Each component is its own lane or console change, merged at a wave boundary.
- Each one follows the chain: red tests first, then the simplifier, the Go and architecture reviewers, and the full floor.
- Q3 to Q11 and Q18 land unwired. Q1, Q12 and Q2 (in `enforce`) change today's ladder and dispatch.
- Q13 to Q17 wire the queue. Q16 lands only after Q10.

The red tests, named by their acceptance criteria:

| # | Red tests |
|---|---|
| Q1 | `TestCatalog_SkillProjectionsAreDerivedOutputs`, `TestCatalog_EveryOutputCarriesItsGeneratedMarker`, `TestCatalog_TheRouterRecipeRegionIsNotAnEntry`, `TestRegionConflict_ABlockInsideTheRegionIsDerived`, `TestRegionConflict_ABlockOutsideTheRegionIsGenuine`, `TestFires_CrossStalenessNeedsBothSides`, `TestRebaseWithDerivedRegen_ACommandStubConflictRegenerates`, `TestRegen_RefusesALeftoverConflictMarker`, `TestRegen_UsesTheGeneratorOfTheWorktree` |
| Q2 | `TestPartitionGraph_ASharedLeafImportIsNotAConflict`, `TestPartitionGraph_TodosInOnePackageShareABucket`, `TestPartitionGraph_TheClosureRelationSeparatesAnImporterFromItsDependency`, `TestPartitionGraph_RepoRelativeCardFilesResolveToTheirPackages`, `TestPartitionGraph_ANewFileResolvesToTheDirectoryPackage`, `TestPartitionGraph_AnIDOnlyTodoStaysAnIsland`, `TestPlanFromTriage_AGoListFailureFallsBackToTheFilePartitionAndWarns`, `TestPlanFromTriage_ShadowEmitsTheGraphPlanAndDispatchesTheFilePlan`, `TestFleetConfig_UnknownPartitionValuesFailSafe`, `TestGlobalZoneFiles_AreRepoRelativeAndHoldTheGitFiles` |
| Q3, the tiers | `TestProof_DisjointPathsAndClosuresAreT1`, `TestProof_ASharedPathIsT3`, `TestProof_APeerChangeInTheLaneClosureIsT3`, `TestProof_ALaneChangeInThePeerClosureIsT3`, `TestProof_ASharedLeafImportIsT1`, `TestProof_ADerivedEntryAloneIsT2`, `TestProof_ADerivedEntryAndAPackageEdgeIsT3`, `TestProof_ABookkeepingOnlyPeerIsT1BeforeStepThree`, `TestProof_BookkeepingNeverRaisesTheTier`, `TestProof_AGenuineConflictIsT4`, `TestProof_UnknownIsNeverT1` (a `rapid` property), `TestEvidenceDigest_DoesNotDependOnInputOrder` (a `rapid` property) |
| Q3, the zones | `TestProof_ABuildZonePathIsT3`, `TestProof_AGateZonePathWidensTheTestsWithoutRaisingTheTier`, `TestProof_AVendoredFileIsInTheBuildZone`, `TestProof_AnEmbeddedFileMapsToItsPackage`, `TestProof_ATestdataFileMapsToItsPackage`, `TestProof_AnUnownedPathUnderGoIsUnknown`, `TestProof_ASkillBodyChangeIsUnknown`, `TestProof_AReadRootIsUnknown`, `TestProof_AProseDocIsNotUnknown`, `TestProof_ANonPlainPathIsUnknownAndT3`, `TestProof_AGraphFailureIsUnknownAndT3`, `TestGraph_IsTheUnionOfTheFourTagSets` |
| Q4 | `TestCompose_ACleanMergeReturnsTheComposedTree`, `TestCompose_UsesTheAttributesOfTheTip`, `TestCompose_ComposesTheAuditedTreeNotShipsCommit`, `TestCompose_AConflictOnANonDerivedPathIsGenuine`, `TestCompose_AConflictInsideAGeneratedRegionIsDerived`, `TestCompose_ExitOneWithNoPathsIsGenuine`, `TestCompose_AnErrorExitTriesOnceMoreThenComposeInfra`, `TestCompose_RefusesABaseThatIsNotAnAncestor`, `TestMaterialize_LeavesTheChangePendingOnTheTip` |
| Q5 | `TestSelect_DisjointChangesSelectOnlyTheirCommonImporters`, `TestSelect_ABookkeepingOnlyPeerSelectsOnlyTheLaneDataReaders`, `TestSelect_TheLaneDataReadersAlwaysRun`, `TestSelect_AnEmbedChangeSelectsItsPackageTests`, `TestSelect_ATestdataChangeSelectsItsPackageTests`, `TestSelect_AnUnresolvedDataRootSelectsTheFullSuite`, `TestSelect_AClosureFailureSelectsTheFullSuite`, `TestSelect_AGlobalZoneChangeSelectsTheFullSuite`, `TestSelect_ADataEdgeAddsTheReadingPackage`, `TestDataEdges_TheCatalogEqualsTheScan`, `TestBookkeeping_NoTestReadsABookkeepingRoot`, `TestApicoverScope_IsTheEnforcedSubsetOfTheSelection` |
| Q6 | `TestQueue_TicketsAreFIFOUnderConcurrentEnqueue`, `TestQueue_OnlyTheHeadLands`, `TestQueue_ALandingNeedsTheTipToEqualMain`, `TestQueue_AnEjectionSendsEveryCandidateBehindItBackToEnqueued`, `TestQueue_TheGreenPrefixLandsPastARedCandidate`, `TestQueue_TheWindowAddsOneOnALandingAndHalvesOnARedEjection`, `TestQueue_AReviewTimeoutKeepsTheWindow`, `TestQueue_AnIffyCandidateGetsASoloSlot`, `TestQueue_ADeadOwnerIsEjectedWithOwnerGone`, `TestDeadOwnerInLanding_NoIntentOrUnwoundEjects`, `TestDeadOwnerInLanding_APreparedIntentPauses`, `TestDeadOwnerInLanding_ACompleteIntentIsLanded`, `TestQueue_AMalformedRecordIsEjectedWithAnIncident`, `TestQueue_EveryTransitionMatchesTheSpecTable`, `TestQueue_ASpentBudgetEjectsWithBudget` |
| Q6, parking | `TestQueue_APauseParksEachOwnerAndReleasesItsLock`, `TestQueue_ALiveOwnerInLandingFinishesInsteadOfParking`, `TestQueue_AParkedRecordIsNotADeadOwner`, `TestQueue_AParkedCandidateIsOutsideTheOrder`, `TestQueue_AReattachKeepsTheTicketAndTheEvidence`, `TestQueue_AReattachAheadChangesThePrefixDigestBehind`, `TestQueue_AReattachInAPausedQueueParksAgain` |
| Q7 | `TestGates_AnEmptySelectionSkipsTestAndApicover`, `TestGates_CompileVetsEveryTagSet`, `TestGates_ACompileRedIsT4BeforeTheRestOfStepThree`, `TestGates_ARegistryConflictIsRed`, `TestGates_TheApicoverScopeIsAMakeParameter`, `TestFlakeScreen_ARedThatPassesAloneIsAFlake`, `TestFlakeScreen_ARedOnTheMainTipPausesTheQueue`, `TestFlakeScreen_ARedOnlyWithTheCandidateEjectsRedGate`, `TestGates_ThePredicateReceiptBindsTheComposedTree`, `TestGates_ATimeoutCountsAsRed`, `TestGates_ABookkeepingOnlyPeerRunsOnlyTheLaneDataReaders`, `TestGates_ResultsCarryAcrossABookkeepingOnlyComposition` |
| Q7, the base checks and the predicates | `TestFlakeScreen_ARedPrefixTipThatIsNotMainKeepsTheCandidateWaiting`, `TestGates_ACompileRedThatTheMainTipSharesPausesTheQueue`, `TestGates_ACompileRedOnlyWithTheCandidateIsT4`, `TestGates_ABookkeepingPeerWithPredicatesCompilesTheAcsTagSet`, `TestGates_ABookkeepingCarryStillCompilesNewPredicates` |
| Q8 | `TestCarrySatisfied_ADerivedRegenRecordIsProvenAgain`, `TestCarrySatisfied_ADerivedRegenRecordWithAStaleOutputIsRefused`, `TestCarrySatisfied_AnInteractionReviewNeedsACompatibleRowForTheDigest`, `TestCarrySatisfied_AReviewOfAnotherDigestIsRefused`, `TestCarrySatisfied_RecomposesAndRefusesAnotherTree`, `TestVerifyPredicateReceipt_AcceptsTheComposedReceiptOfAQueueRecord`, `TestCarrySatisfied_AcceptsGatesCarriedAcrossBookkeeping`, `TestLedger_TheNewMethodsSurviveTheRoundTrip` |
| Q9 | `TestRebind_ARegeneratedCommandStubKeepsTheIdentity`, `TestRebind_AHandEditedDerivedOutputStillDeclines`, `TestRebind_AByteChangeOutsideTheCatalogStillDeclines` |
| Q10 | `TestReviewPrompt_HoldsTheEvidenceTheLaneDiffAndThePeerDiffInOrder`, `TestReviewPrompt_NeverCutsTheEvidenceBlock`, `TestReviewPrompt_MarksTheBytesCut`, `TestReviewPrompt_ForbidsAReAuditOfTheLane`, `TestReviewReport_AMissingEvidenceRowIsUnsure`, `TestReviewReport_ABindingMismatchIsUnsure`, `TestReviewReport_AHighFindingIsAnInteractionDefect`, `TestReviewReport_MediumFindingsBecomeInboxFollowUps`, `TestReview_ATimeoutIsUnsure`, `TestRouting_TheLandingReviewerIsInTheClaudeFamilyFloor`, `TestRouting_TheLandingReviewerIsOpusAtMediumEffort` |
| Q11 | `TestRegistry_TheQueueCodesAndKindAreRegistered`, `TestSignalsCodesCheck_FailsOnARegistryConflictInTheFullBinary`, `TestRecover_EachLandingEjectionRoutesToItsPhase`, `TestRecover_PreconditionReauditStillTakesTheOtherPreconditions`, `TestRecover_LandingQueuePausedRoutesToEnd`, `TestFloor_LandingQueuePausedHaltsTheLoopAsInfraSystemic` |
| Q12 | `TestRouteRebasedExplanation_ACommittedChangeReportsTheProofSkipped`, `TestRecoverFromShipError_AContinuationLaneRunsTheIdentityProof`, `TestRecoverFromShipError_AContinuationLaneKeepsItsReleasedContinuation`, `TestRebind_TheSanctionedConsumptionKeepsTheIdentity`, `TestRebind_AnyOtherInboxChangeStillDeclines` |
| Q13 | `TestShadow_RecordsTheTierBesideTheRouteAndChangesNothing`, `TestShadow_SpendsNoLLMPhaseAndRunsNoGate`, `TestShadow_AProofFailureIsRecordedAndNotFatal`, `TestShadow_KeepsTheComposedTreeOfAT1OrT2Decision`, `TestShadowVerify_RunsTheGatesAndTheCISuiteOnEachKeptTreeAndTheSuiteOnItsTip`, `TestShadowVerify_AnEscapeNeedsGreenGatesARedSuiteAndAGreenTip`, `TestShadowVerify_DeletesEachRefAfterItsCheck`, `TestWaveNext_RunsShadowVerifyInTheBoundaryWhileTheStageIsShadow` |
| Q14 | `TestEnforce_ThreeDisjointLanesLandInTicketOrderWithNoLLMPhase`, `TestEnforce_ACodeOverlapWithReviewAuditEjectsToAudit`, `TestEnforce_AConflictEjectsAndTheGreenPrefixLands`, `TestEnforce_AContinuationLaneLandsWithNoBuild`, `TestEnforce_ADossierCommitOnMainComposesAgainAsT1`, `TestEnforce_AQueueLaneTakesNoShipWindowLease`, `TestEnforce_ARedTipHaltsTheLoop`, `TestEnforce_APausedQueueHaltsTheLoopAndTheResumeReattaches` |
| Q15 | `TestEnqueue_TheOwnerLockIsHeldBeforeTheRecordIsVisible`, `TestWait_TheWaiterReleasesTheLockAtOnce`, `TestWait_AWaiterWakesOnlyWhenTheCandidateAheadIsTerminalOrDead`, `TestWait_ALivenessCheckNeverKeepsTheLock`, `TestWait_TheDeadlineWarnsAndKeepsWaiting`, `TestWait_AWaitingLaneNeverPolls` (the AST guard of §8), `TestQueue_APausedQueueHasNoWaiterThatPolls`, `TestWait_AParkingWakesTheWaiterBehind` |
| Q16 | `TestOrchestrator_ShipGoesToLandingReviewAndBack`, `TestOrchestrator_ReviewAuditHasNoReviewEdge`, `TestEnforce_ACodeOverlapRunsOneReviewAndLands` |
| Q17 | `TestLandingQueueCLI_StatusPrintsEachCandidate`, `TestLandingQueueCLI_ResumeRefusesWhileTheCauseHolds`, `TestLandingQueueCLI_ResumeLandsAStrandedCommitThatOriginHolds`, `TestLandingQueueCLI_ResumeSettlesLocalMainToOrigin`, `TestLandingQueueCLI_ResumeParksAStrandedCommitThatOriginLacks`, `TestLandingQueueCLI_EjectRoutesAsReviewUnsure`, `TestLandingQueueCLI_EjectEndsAParkedRecord`, `TestLandingQueueCLI_ShadowVerifyReportsEachEscape` |
| Q18 | `TestCensus_ReproducesTheRecoveryTableOfTheDossier`, `TestCensus_TiersComeFromTheProof`, `TestCost_ACycleCostIsTheSumOfItsRecoveries`, `TestReplay_ServesArrivalsInOrderWithAFixedStep` |
| Q19 | the deletion of each retired path with its tests; `TestPerLane_ARebaseStillRoutesToAFullReAudit` (preservation) |

## 8. TDD protocol

The rules for each component:

1. Write one test for one acceptance criterion. See it fail on its named assertion, and keep the red output in the lane scratchpad.
2. Write the smallest code that passes it.
3. Run mutants of each fix in a scratch copy: reorder, shift, wrong key, wrong tense and a hard-coded input.
4. Unit tests fake the clock, the git runner, the `go list` runner, the lock and the review dispatch. No test sleeps.
5. Each git fixture uses `internal/gittest`, never a raw `git init`.
6. New packages reach 100% line and API coverage. Changed functions in mixed packages reach 100% each.
7. Run `go test -count=1 -race` and the full floor before the review.

The proofs that drive real processes:

- **The ticket race** runs the test binary again N times, as `TestAppend_TwoProcessStress` does (`go/internal/adapters/ledger/ledger_crossproc_test.go`).
- **The compositions** use real repositories with `merge=union`, region files and real conflicts, as the probe of research F9.2 does.
- **The 1843 replay** builds a continuation lane whose consumed item releases a continuation. It must land with no Build in `enforce` (Q14), and it must reach the proof in today's ladder (Q12).
- **The waits** hold no `time.Sleep`, `time.Tick` or `time.NewTicker`. An AST guard, the walker of `go/internal/sysexec/command_guard_test.go`, checks the queue code.

## 9. Migration from the current ladder

| Part | Today | With the queue | When |
|---|---|---|---|
| `Landing.CheckFastForward` (raise point) | raises `GIT_FLEET_REBASE_NEEDED` | kept; for a queue lane, a divergence means that another writer moved `main`, so the head composes again | none |
| `recoverShipError` (hook) | routes ship errors | kept, for `per-lane` and for the ejection codes | none |
| `shipRecoveryBudget` | scales by width | kept; each ejection costs one unit | none |
| `contentionBackoff` | jitter before a recovery | kept for `per-lane`; the queue has no race | none |
| Pre-screen `ClassifyFleetRebaseCandidate` | landed, clean or conflict | kept for `per-lane`; the proof replaces it for queue lanes | stage 7 |
| B1 unwind | unwinds ship's commit | retired for queue lanes, which compose `T0`; `per-lane` loses it at stage 7 too (CD3) | stage 7 |
| `rebaseWithDerivedRegen` | rebase, with `control-flags.md` regeneration | rewired to the catalog (Q1); kept for `per-lane` | stage 1 |
| B2 `RebindIdenticalRebase` | the rebind after a rebase | kept and reused (T1, T3); extended for T2 (Q9) | stage 5 |
| B3 `identityCarryForward` | the full gates and the carry record | retired for queue lanes, where the queue writes the record with scoped gates; `per-lane` loses it at stage 7 too | stage 7 |
| The interim fix (Q12) | the identity proof for continuation lanes | deleted with B1 and B3 | stage 7 |
| B4 `carrySatisfied` | ship accepts a carry | kept, with two new methods (Q8) | stage 5 |
| B5 `resumeFleetRebaseAfterDebugger` | the debugger re-entry | a queue lane enqueues again after the debugger | stage 7 |
| RUNG 0 `compositionCarryForward` | unreachable for contract cycles | retired | stage 7 |
| RUNG 2 `scopedMergeCarryForward`, `mergerung2.go` | never wired | retired | stage 7 |
| `composedGatesTo` (four full `make` targets) | the carry gates | replaced by the scoped gate set (Q7) | stage 7 |
| Router rule `fleet-rebase-reaudit` | sends the rebase to Audit | kept for `per-lane` | none |
| The ship-window lease | serializes binding to push | kept for `per-lane`; a queue lane in `enforce` takes none (D35) | stage 5 |
| `ship.lock` | the git critical section | kept | none |
| The two-phase landing (ADR-0039 §8.1) | intent, push, settle | kept unchanged; the head lands through it | none |
| `fleet.PrefixQueue`, `PlanLanding`, `LandPrefixes` | no production caller | retired; `internal/landingqueue` replaces them | stage 7 |
| `fleet.landing` resolution | `per-lane` or `prefix-queue` | kept; it gains the `landing_queue` block | stage 4 |
| `PartitionGraph` | unwired; transitive sets meet | narrowed and wired (Q2) | stage 2 |
| `derivedArtifacts` map | one entry | replaced by `internal/derived` | stage 1 |
| `changedpkgs`, `regressiontia` | the ship backstop, a shadow selection | reused by the selection (Q5) | stage 3 |
| `fleet.GlobalZoneFiles()` | one list with two path bases | a build zone and a gate zone, relative to the repository root (D31) | stage 2 |
| `evolve signals codes check` | checks the drift of the generated region | also fails on a registry conflict (Q11) | stage 3 |

At stage 7, `per-lane` loses B1 and B3 as well. It keeps only the full re-audit route of the router rule `fleet-rebase-reaudit`, and the interim fix of Q12 goes with the ladder.

## 10. Rollout

| Stage | Content | Entry | Exit |
|---|---|---|---|
| stage 0 | Q0: the docs | the operator's choice | this lane merged |
| stage 1 | Q1 and Q12: the derived catalog and the interim fix, live at once as fixes of today's ladder | stage 0 | 2 waves with no continuation lane on the Build route, and no derived conflict at the debugger |
| stage 2 | Q2 in `shadow`, then in `enforce` | stage 0 | the shadow width cost is at most 10% of the lane slots over 5 waves |
| stage 3 | Q3 to Q7, Q11 and Q18, unwired | stage 0 | coverage at 100 and the full floor green; Q18 reproduces the tables of research §11 |
| stage 4 | Q13 and Q17: `fleet.landing=prefix-queue` with `stage: shadow` | stage 3 | 20 fleet rebases or 2 weeks, with the shadow metrics of §11 |
| stage 5 | Q8, Q9, Q14 and Q15: `stage: enforce` with `review: audit` | the stage 4 metrics hold (next list) | the exit criteria below, over 20 landings |
| stage 6 | Q10 and Q16: `review: interaction` | the stage 5 exit holds | 10 reviews with zero escapes after `COMPATIBLE`, and a measured token saving (below) |
| stage 7 | Q19 and the default flip (D33) | stage 6, or stage 5 if `review` stays `audit` | the exit criteria below hold for 20 more landings |

The stage 4 metrics that open stage 5:

- the selection recall is 100%: each package that today's full gates found red is in the selection `S`;
- `shadow-verify` found zero shadow escapes (spec §14);
- no shadow T1 or T2 decision met an Audit FAIL in today's route, other than a known false red of a gate;
- the proof reports `unknown` for at most 10% of the rebases.

The token saving of stage 6: the mean tokens of a `landing-review` stay below the mean tokens of an Audit on `C` in stage 5. If the saving does not show, `review` goes back to `audit`. The model gives stage 6 more minutes than stage 5 (research §11), so tokens and caught interactions are its only gains.

The exit criteria of stages 5 and 7. The model expects a mean of 8.8 min and a median of 9.2 min at stage 5, and 10.4 and 12.9 at stage 6:

- the mean minutes per rebase are at most 12;
- the median minutes per rebase are at most 14;
- the LLM phases per rebase are at most 0.8 (the model expects 0.66);
- zero false-carry escapes;
- each pause ends through `resume` and the cycle resume, and each parked candidate re-attaches with its ticket.

## 11. Metrics

| Metric | Definition | Source |
|---|---|---|
| Minutes per rebase | Today: from `ship.error` with `SHIP_GIT_FLEET_REBASE_NEEDED` to the next ship PASS. Queue: from the first `SHIP_QUEUE_COMPOSED` with a peer delta to `SHIP_QUEUE_LANDED`. | the per-cycle `signals.ndjson` |
| LLM phases per rebase | The count of Build, Audit, debugger, TDD and `landing-review` outcomes in the same window | `phase-timing.json` |
| False-carry escapes | A landing with no review (T1, T2) or with `COMPATIBLE` (T3), then a bad sign within 24 h (next list) | `ci.completed` (ADR-0127), git, the inbox |
| Shadow escapes | A kept T1 or T2 tree that passes the gate set and fails the CI suite, on a green tip (spec §14) | `landing-shadow.json`, `SHIP_QUEUE_SHADOW_ESCAPE` |
| Shadow Audit disagreement | A shadow T1 or T2 decision where today's route ran an Audit that returned FAIL for a reason other than a known false red | `landing-shadow.json`, the audit reports |
| Review tokens | The tokens of each `landing-review`, against the tokens of each Audit on `C` in stage 5 | `<phase>-usage.json` in the run directory |
| Selection recall | In shadow, the share of the red packages of today's full gates that `S` selected | `landing-shadow.json`, `ORCHESTRATOR_COMPOSED_GATE_DECLINED` |
| Queue wait | From the enqueue to the first composition at the head, less the candidate's own work | the record times |
| Tier mix | The share of T1, T2, T3 and T4, and of the rules that fired | `SHIP_QUEUE_TIER` |
| Ejection mix | The share of each ejection reason | `SHIP_QUEUE_EJECTED` |
| Flake rate | Gate reds that passed alone, against all gate reds | `SHIP_QUEUE_GATE_FLAKE` |
| Pauses | The count and the length of each pause, and the parked candidates that re-attached | `SHIP_QUEUE_BASE_RED`, `SHIP_QUEUE_HEAD_STRANDED`, `SHIP_QUEUE_PARKED` |

A bad sign for the false-carry metric is one of these:

- a red `ci.completed` on the landed commit, for a test that passes on its parent;
- a revert of the landed commit;
- a defect item that names the landed cycle and a peer cycle.

`evolve wave status` gains one line for these metrics, read from the signals and never from the text log.

## 12. Verification

- **Each component:** its red tests (§7), mutants, 100% coverage, and `go test -count=1 -race`.
- **Each landing:** the full floor (test, integration, e2e, acs, apicover and cover-strict) before the merge.
- **The replays:** the 1843 replay (§8) runs in both Q12 and Q14. The census of research §15 (Q18) runs over the runtime runs, and it must match the shadow tiers of the same cycles.
- **Live, stage 4:** a real wave in shadow writes `landing-shadow.json` for each fleet rebase, and it changes no route.
- **Live, stage 5:** a real wave in `enforce` lands three lanes in ticket order. A disjoint lane lands with no LLM phase.

## 13. Patterns and forces

| Pattern | Where | Force |
|---|---|---|
| Specification and chain of responsibility | the tier rules: each rule is a predicate over the evidence, and the strictest tier wins | One home for the rules; a new rule adds a predicate, not an edit to a switch. |
| State machine as a table | the candidate lifecycle | One transition table, tested against the spec table. |
| Ports and adapters | git, `go list`, the clock, the lock, the gate runner and the review dispatch | The proof and the queue model stay pure, so tests fake only process boundaries. |
| Strategy | the record method for each tier, in the writer and in ship's proof | Two sites switch on the tier, so one strategy for each tier. |
| Single source with projection | the derived catalog and its four readers | One home, pinned by drift tests. |
| Repository | the queue store behind a port | Atomic writes; an in-memory store tests the lifecycle. |
| Observer | the signals | They observe and never decide. |

The forces come from the research (§12): cost, safety, liveness, throughput, determinism and the conservation of proven work.

Not abstracted:

- The tiers, the states and the ejection reasons are data, not types.
- No broker, no daemon and no ML model.
- No interface for a single implementation. The store port has two real implementations: the files and the in-memory store.

## 14. Risks

The limits of record are in the spec ([fleet-landing-queue.md](../architecture/fleet-landing-queue.md), Limits). The risks of the rollout:

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| The proof misses a semantic interaction, so T1 lands a defect. | low: 1 of 417 mined pairs interfered (F7.3) | a red `main` or a latent defect | the scoped gates, the predicates on `C`, the CI on `main`, the shadow oracle, the escape metric, and T3 for each package edge |
| The selection misses a red, from a test that is not hermetic. | medium | a red `main` | the data-edge guard, full suites for unresolved roots, ship's pack at the landing, and the recall metric in shadow |
| A new production read of a prose path stays outside the read roots. | medium | T1 lands a data interaction | the CI on `main`, the escape metrics, and the catalog of production reads (later work) |
| The review deadline ejects slow but correct reviews. | medium | more Audits on `C` | the review durations of stage 6, and a tuned `review_timeout_minutes` |
| Head-of-line wait slows the landings. | low: a mean of 0.53 min in the replay | slower landings | the window, and a WARN at `max_wait_minutes` |
| A pause stalls the fleet. | low | a halted loop | each owner parks, and the loop halts with a P0 item (ADR-0072); `resume`, then the cycle resume, re-attaches each candidate with its ticket (D47) |
| A review says `COMPATIBLE` on a real defect. | medium | a defect lands | the gates, the CI on `main`, `review: audit` first, and the escape metric |
| The protected surface changes in several packages. | certain | review cost | console work at boundaries, small commits, and the architecture reviewer |
| Two landing paths live side by side until stage 7. | certain | maintenance | shadow first, then one retirement at the flip |
| Other writers keep moving `main`. | high: 53 of 119 commits were dossiers | more compositions | a cheap T1 composition; CD2 |
| T3 is common at package granularity. | high: 19 of 39 | review cost | the `package` dispatch rule, and the measurement in shadow |

## 15. Open questions

None. On 2026-10-09 the operator decided OQ1, OQ3, OQ4 and OQ6 (§4, O10 to O13). The console decided OQ2, OQ5, OQ7 and OQ8 on their recommendations (§4, CD1 to CD4), and the operator can override each one.

## 16. Status

| # | Status |
|---|---|
| Q0 | ◐ the dossier, ADR-0128, the spec and this plan are written in `dev/cl-landing-queue`, with the operator decisions of 2026-10-09 and review fix round 1; not staged |
| Q1 to Q11, Q13 to Q19 | ☐ not started |
| Q12 | ◐ built in `dev/cl-lq-q12`, with the five named tests and the 1843 replay; not staged. The route is Audit, not Ship (D53). |
