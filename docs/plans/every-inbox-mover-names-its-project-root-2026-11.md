# Plan: every inbox mover names its project root

> Type: plan (design before build). Item: `every-inbox-mover-names-its-project-root` (rank 49, route console-manual). Spec of record: [internal-inboxmover.md](../architecture/packages/internal-inboxmover.md). Decision records: [ADR-0079](../architecture/adr/0079-cycle-outcome-inbox-lifecycle-seam.md).

## Request

The item asks for "a design that passes architecture review". Five review rounds of two designs (root inference, and explicit root plus a guard) failed. The reviews found these faults:

- pre-resolving facades;
- continuation-registry writers;
- `Roots.ProjectRoot` beside `Roots.EvolveDir`;
- two `Claimer` callers;
- a syntax-only guard;
- a CI allow list that cannot be pinned.

This plan answers each finding. It also answers the second review round, which is quoted in the section "Review findings and answers".

## Terms used throughout

- **Plane root**: the directory that holds `.evolve/`. Code names it `projectRoot`.
- **Mover**: a call into `internal/inboxmover` that reads or changes the inbox lifecycle.
- **Treewalk guard**: `evolve treewalk snapshot` and `check`. It fails a run when tests leave a file or an empty directory in `go/`.

## Findings

**F1.1** `Options.resolveOpts` sets `ProjectRoot` from `InboxDir` through `planeRootOf` (`go/internal/inboxmover/inboxmover.go:89-92`). `planeRootOf` returns `""` when `InboxDir` does not end in `.evolve/inbox` (`inboxmover.go:80-87`). `holder.go:65-68` has a second, unconditional inference: `projectRoot = Dir(Dir(InboxDir))`.
*Implication:* the root is a second belief, derived from a path shape in two places. The shape can fail.

**F1.2** With `ProjectRoot == ""`, the ledger path becomes `.evolve/ledger.jsonl` and `ActiveCycleFn` reads `.evolve/cycle-state.json`. Both are relative to the working directory (`inboxmover.go:96-97,110`). The two git probes run in the working directory (`inboxmover.go:114-122`). `releaseContinuationOnRetire` skips its work without a sound when the root is empty (`continuation_retire.go:24`).
*Implication:* a mover with an unusual `InboxDir` writes the lifecycle row, and probes landing, in the wrong tree. No error appears.

**F1.3** Nine production literals pass only `InboxDir` (checked 2026-11-21): `go/cmd/evolve/cmd_loop_goal_ids.go:126`, `cmd_supply_wiring.go:64`, `cmd_cycle.go:749` (`hostInboxClaimer`) and `:870` (`laneMenuOf`), `go/internal/triagecap/lane_menu.go:89`, `go/internal/phases/triage/triage.go:189`, `go/internal/deliverable/effects.go:73`, `go/internal/cli/inboxcmd/cmd_inbox.go:136` and `cmd_inbox_trailercheck.go:47`.
*Implication:* only `cmd_cycle.go:749` writes (`ClaimPending`). The other eight call read-only facades (`PlaceOnLaneMenu`, `PartitionLaneMenu`, `RankLaneMenu`, `ResolveDispatchability`, `ResolveDispatchState`). Those facades use only `opts.InboxDir` (`dispatchability.go:18-77,93-138`, `dispatchstate.go:30-51`). They never touch the ledger, `ActiveCycleFn`, the git probes or the registry, and `ledger.New` does no I/O (`adapters/ledger/ledger.go:77-88`). The fault cannot occur on those eight.

**F1.4** `make test` runs the treewalk guard (`go/Makefile:107-112`). `make test-integration` does not (`go/Makefile:133-178`). CI runs `make test-integration` (`.github/workflows/go.yml:51`). The open, lane-routed item `ci-test-integration-treewalk-parity` owns the Makefile change.
*Implication:* a leak from an integration-tagged test passes CI. This plan does not change the Makefile.

**F1.5** The path home is `paths.EvolveDirOf(projectRoot)` (`go/internal/paths/paths.go:113`). It has no inverse. Hand inversions exist at `cmd_loop_escalate.go:56`, `cmd_loop_wave.go:77,81`, `cmd_cycle_start.go:39`, `cmd_ledger.go:203`, `deliverable/grammars.go:49`, `deliverable/catalogaware.go:21`, `triagecap/lane_menu.go:89`, `inboxcmd/cmd_inbox_consume.go:114`, `inboxmover/holder.go:67`, and the shape check in `router/digest.go:410-422`. `planeRootOf` is one more copy of the `.evolve` shape rule.
*Implication:* the "root from evolve dir" rule has no single home.

**F1.6** Each exported facade has one of four roles. The table lists each one (checked 2026-11-21).

| Facade | Reads the root | Writes | Error return today | On a refusal |
|---|---|---|---|---|
| `Claim`, `Promote`, `ReleaseFromQuarantine`, `RecoverOrphans`, `ReleaseCycleProcessing*`, `ReconcileSuperseded`, `RouteLane*`, `RouteConsole*`, `File`, `Edit*`, `Withdraw`, `VerifyPremise`, `AbsorbRootCopies` (all through `mover()`, `inboxmover.go:186-190`) | yes | yes | yes | return the error |
| `ClaimPending`, `ClaimDispatchable`, `SurveyClaims`, `ReleaseClaim`, `ReleaseClaimKeeping`, `ReleaseStaleClaims`, `ReleaseContinuationBinding`, `ReleaseContinuationBindingFor`, `ShedDeclinedContinuation`, `ApplyCycleOutcome`, `ConsumeFixItem`, `RetireUnbacked` (direct `resolveOpts()` calls) | yes | yes | yes | return the error |
| `ResolveContinuationForScope` (`continuation_resolve.go:72`), `ReconcileConsumedBindings` (`continuation_release.go:117`) | yes (registry, `:81`, `:133`) | yes (they delete dead bindings) | no | gain an error return (D2) |
| `ClaimLaneScope` (`outcome.go:141`) | no own read | only through `Claim` | no | no change: `Claim` refuses, `ClaimLaneScope` logs a WARN and claims nothing (`outcome.go:150-152`) |
| `ResolveContinuation` (`continuation_resolve.go:15`), `ClassifyHolder` (`holder.go:44`), `FindScopeItemFile`, `FilterPromotionScope`, `ReadFailureCount`, `ReadLastFailureClass`, `ResolveDispatchability`, `PendingDispatchability`, `ResolveDispatchState`, `PlaceOnLaneMenu`, `PartitionLaneMenu`, `RankLaneMenu` | no (they read `InboxDir` only) | no | no | no change |

`ClassifyHolder` has callers only in `claims_test.go`. `ReleaseContinuationBindingFor` (`continuation_release.go:25-52`) reads and deletes registry entries under `opts.ProjectRoot` and never reaches `mover()`.

`ResolveContinuationForScope` has one production caller (`cmd_cycle.go:733-735`), and `ReconcileConsumedBindings` has one (`cmd_inbox_consume.go:207-208`). Both callers name only `ProjectRoot`.
*Implication:* a refusal belongs to the first three rows. Only two facades gain an error return, and each has one production caller. The last two rows cannot fault, and the spec says `ReadFailureCount` "does not refuse" (`internal-inboxmover.md:17`).

**F1.7** The root has three sources in the cmd layer. `--project-root` and `--evolve-dir` are separate flags (`cmd_cycle.go:126,201`, `cmd_loop_args.go:53,196-198`). `evolve inbox mover` and the curator read `EVOLVE_PROJECT_ROOT` or the working directory (`internal/cli/inboxcmd/cmd_inbox_mover.go`, `cmd_inbox_curator.go:114`).
*Implication:* a root and an inbox directory can disagree. That disagreement is the mixed-tree write that the item names.

**F1.7a** Two production literals set the root and the inbox from separate inputs. `loopwave/launcher.go:47` is fed by `cmd_loop_wave.go:26` (`Roots{cfg.ProjectRoot, cfg.EvolveDir}`). `cmd_cycle_lane_resume.go:183` sets `ProjectRoot` and `inboxDir()`. A fleet-only check of the same rule exists: `fleetLaneEvolveDirOK` (`cmd_cycle_fleet_evolvedir.go:11`), and it runs only when the fleet environment value is set.
*Implication:* the mismatch is reachable in production. The resolver is the only check that every path crosses.

**F1.8** `TestMutators_AnExplicitPlaneInboxFromACycleWorktreeIsAllowed` and spec line 17 allow `ProjectRoot` = a cycle worktree with `InboxDir` = the plane inbox. The plane items move, while the ledger, the cycle state, the git probes and the registry resolve under the worktree.
*Implication:* this is the mixed-tree write, allowed on purpose. The design must decide it.

## Root cause

`Options` has two inputs for one fact (`ProjectRoot`, `InboxDir`). Two places infer one from the other (`planeRootOf`, `holder.go:65-68`), and a third path lets both be set without a check. For the root-dependent facades, a wrong or empty root writes in the wrong tree, and nothing fails.

## Approaches

The fault occurs only in the first three rows of the F1.6 table, and only for an off-shape `InboxDir` or a root/inbox mismatch.

| Criterion | A: one resolver, fail loud, refuse a mismatch | B: explicit root at nine sites, new `LaneMenuFn` and `readOnlyLifecycle` plumbing | C: new constructor `inboxmover.ForRoot`, unexported `InboxDir` | D: add `Roots.ProjectRoot` to `phasecontract.Roots` |
|---|---|---|---|---|
| Answers the review findings | Yes (table below) | Yes, with the same refusal | Yes | No. Review named `ProjectRoot` beside `EvolveDir` as a fault. |
| Fault cannot occur | Yes. Off-shape and mismatch both refuse. | Yes, same as A | Yes, by type | Yes, for deliverable only |
| Blast radius | 1 resolver, 2 facades with a new error return (1 production caller each), 3 hand inversions, and the tests that the draft run lists (D6) | About 40 more type and test edits. No added safety, because the readers cannot fault. | A + 102 test literals and 48 production literals | 40+ `Roots` users |
| One home per rule | `paths.ProjectRootOf`, called by the resolver | Same | `ForRoot` | Two root fields |
| Proof is behavioural, not syntactic | Yes | Yes | Type system | Yes |
| Feature flag or env control value | None | None | None | None |

**Why A wins.** The readers cannot fault, so B and C buy no safety for their cost. D repeats a rejected design. A is the simplest design that works: one resolver and one mismatch rule.

Two earlier parts are cut. The `Claimer` signature stays as it is, because a root-typed `Claimer` has the same Go type as the old one. The compiler cannot find a stale caller. The flag-level check moves to a named follow-up (see "Follow-up items").

## Decisions

| Id | Decision | Because | Rejected |
|---|---|---|---|
| D1 | `paths.ProjectRootOf(evolveDir string) (string, bool)` is the inverse of `EvolveDirOf`. It returns `("", false)` when the base name is not `.evolve`. `planeRootOf` keeps its `inbox` check and calls it. `cmd_loop_escalate.go:56` and `inboxcmd/cmd_inbox_consume.go:114` move to it too. D2 deletes the `holder.go:65-68` inference. A follow-up item owns the other inversions, and no count pin exists. | F1.5. | A count pin of the spelling `filepath.Dir(evolveDir)`: it checks syntax only, and a renamed variable defeats it. Claim all F1.5 sites here: it widens the item with no fault to fix. |
| D2 | `Options` gets one resolver: `func (o Options) resolved() (Options, error)`. The steps are in "The resolver steps". The first three rows of the F1.6 table call it first. `ResolveContinuationForScope` and `ReconcileConsumedBindings` gain an `error` return. `holder.go` uses the resolved root, and the skip at `continuation_retire.go:24` goes. The other facades keep `resolveOpts()`. | F1.1, F1.2, F1.6, F1.7a | Return an error from every facade: the readers cannot fault, and about 40 callers and tests change. Compare raw strings: `TestCycleWorktreeRefusal_AnUncleanInboxPathStillRefuses` expects the worktree error. |
| D3 | The mixed tree is refused. `ProjectRoot` = a cycle worktree with `InboxDir` = the plane inbox is now an `errRootInboxMismatch`. `TestMutators_AnExplicitPlaneInboxFromACycleWorktreeIsAllowed` is rewritten: the allowed row names the plane root, and the worktree-root row expects the refusal. Spec lines 17 and 19 change to match. | F1.8. The plane rows must not move while the ledger and the registry resolve in another tree. | Keep the mixed tree allowed: it is the fault of the item. |
| D4 | The `Claimer` type stays `func(inboxDir string, cycle int, ids []string) error`. `hostInboxClaimer` (`cmd_cycle.go:746-750`) passes an `InboxDir`-only `Options`, which D2 resolves through `planeRootOf` or refuses. The red test shows that `hostInboxClaimer` refuses an inbox that is not `<root>/.evolve/inbox`. | A root-typed `Claimer` has the same Go type as the old one. The compiler cannot find a stale caller, and the double sites compute `ledger.New(filepath.Dir(inboxDir))`. D2 already gives the same safety. | A `Claimer` of `(projectRoot, ...)`: no added safety, and about 17 test and double sites change. A named `ProjectRoot` type: it adds a type for no fault. |
| D5 | The guard is behavioural. A new table test lists each facade of the first three F1.6 rows, and gives each an off-shape inbox and a mismatch. A completeness test parses the package with `go/parser`. It requires each exported function that takes an `Options` to be in the refusal table or in a named no-refusal list. A new facade then fails the test until its author picks a row. | The syntax scan checks only that the table is complete. The rows prove the behaviour. | A `go/ast` scan for `inboxmover.Options{`: it passes a literal that sets a wrong root. A private `resolvedOptions` type: see "Follow-up items". |
| D6 | The build step starts with a draft run. Write the resolver, run `go test -count=1 ./...` from `go/`, and write each failing test into the section "Draft-run list" of this plan. Update each such test to name a root, and keep the test intent. | A single-line grep gave the figure of 13 literals that name no root, and it is wrong. Four of the 13 name a root on a later line (`cmd_loop_escalate.go:55`, `cycleoutcome.go:64`, `inboxcmd/cmd_inbox_mover.go:14`, `postship.go:128`). The tests carry the risk: `cmd/evolve` has 85 `evolveDir := t.TempDir()` fixtures. | A census by grep: it misses the multi-line literals. |
| D7 | This plan does not touch `go/Makefile` or `go.yml`. The item `ci-test-integration-treewalk-parity` owns the CI change. The operator decides, with the boundary owner `console`, to replace the fix of that item with the mechanism in "CI half". The inbox item title is narrowed to the mover root. The PR uses `Refs-Inbox` for the CI half and `Closes-Inbox` only for the mover half. | F1.4. `test-integration` writes `coverage.run.*`, `coverage.partial.txt`, `coverage.txt`, `bin/coversplit`, `bin/testlatency`, `bin/testrusage`, `go/test-integration.json` and `go/test-rusage.ndjson` under `go/`, and treewalk has no exclusions (`treewalk.go:26-38`). A snapshot at `go/` with no allow list reports a leak on every green run. | Build the wrap here: two items then own one change. Keep the allow list of the other item: round 1 rejected an allow list that cannot be pinned. |

### The resolver steps

`resolved()` runs these steps in order.

1. An empty root takes `planeRootOf(InboxDir)`. So `<root>/.evolve/queue` still gives no root.
2. A root that is empty or not absolute gives `errNoProjectRoot`. A relative `InboxDir` gives a relative root, so it refuses here.
3. An empty `InboxDir` becomes `<root>/.evolve/inbox`.
4. A `filepath.Clean(InboxDir)` that differs from `filepath.Join(paths.EvolveDirOf(root), "inbox")` gives `errRootInboxMismatch`. The test compares cleaned paths, so a trailing `/.` does not trigger it.
5. A set `LedgerPath` with a nil `Ledger` gives `errRootInboxMismatch` when its directory is not `paths.EvolveDirOf(root)`. The two production literals that set `LedgerPath` to its default (`loopwave/launcher.go:47`, `cmd_loop_escalate.go:58`) drop the field.
6. `cycleWorktreeRefusal` runs last.

The new messages name `--project-root`. The advice of `inboxmover.go:183` to set `EVOLVE_PROJECT_ROOT` becomes `--project-root` too, because the repository reads no control value from the environment.

### CI half

At the boundary, `console` changes the fix of `ci-test-integration-treewalk-parity` to this mechanism. The recipe scratch (`coverage.run.*`, `coverage.partial.txt`) moves to a `mktemp` directory outside `go/`; `TEST_JSON` and `TESTRUSAGE_OUT` point outside `go/` in all five `go.yml` jobs. `TEST_JSON` is at lines 44, 179 and 257, and `TESTRUSAGE_OUT` is at lines 46, 90, 181, 215 and 259. One tested verb, for example `evolve treewalk run --root . -- <cmd>`, does the snapshot, the command and the check. `make test` and `make test-integration` both call it, and that removes the copy of the inline shell at `Makefile:107-112`.

## Worked example

`evolve inbox list` runs from `/tmp/x` with `--project-root /plane`. Today `cmd_inbox.go:136` builds `Options{InboxDir: /plane/.evolve/inbox}`. This is a read-only call, and it stays unchanged.

A `ClaimPending` call with `Options{InboxDir: /plane/custom-inbox}` gets `ProjectRoot ""` today, and the lifecycle row goes to `/tmp/x/.evolve/ledger.jsonl`. After the change, `resolved()` returns `errNoProjectRoot` and nothing is written. A call with `ProjectRoot: /plane` and `InboxDir: /other/.evolve/inbox` returns `errRootInboxMismatch`.

## Files to change or add

- `go/internal/paths/paths.go`: add `ProjectRootOf`. This file is not in the protected surface: `integrity_surface.go` protects only `paths/seat`.
- `go/internal/inboxmover/inboxmover.go`: add `resolved()`, `errNoProjectRoot` and `errRootInboxMismatch`. `planeRootOf` calls `ProjectRootOf`. Change the message at `:183`. This file is protected, and the item is console-manual for that reason.
- `go/internal/inboxmover/holder.go`, `continuation_release.go`, `continuation_resolve.go`, `continuation_shed.go`, `continuation_retire.go`, `claim_pending.go`, `claims.go`, `retire_unbacked.go`, `fix_item.go`, `outcome.go`: call `resolved()` where the F1.6 table says so. Delete the empty-root skip at `continuation_retire.go:24`.
- `go/cmd/evolve/cmd_cycle.go:733` (`continuationForScope`) and `go/internal/cli/inboxcmd/cmd_inbox_consume.go:207`: handle the new error return (see "Signals").
- `go/cmd/evolve/cmd_loop_escalate.go:56-58` and `inboxcmd/cmd_inbox_consume.go:114`: use `ProjectRootOf`. Drop `LedgerPath` at `cmd_loop_escalate.go:58` and `loopwave/launcher.go:47`.
- The tests of the "Draft-run list", including the ACS predicate `go/acs/cycle308/reproduce_cycle308_test.go:13`. It calls `ReleaseCycleProcessing(Options{}, 0)` and expects no error. Decision: update it to pass `Options{ProjectRoot: t.TempDir()}` with a note, because the predicate proves that the release API exists, and that intent holds.
- `docs/architecture/packages/internal-inboxmover.md`: the root rule in Design and Invariants. Rewrite lines 17, 19 and 22 (the worktree rule, the pinned tests and the `planeRootOf` fallback).
- `docs/architecture/packages/internal-paths.md`: `ProjectRootOf`.
- `docs/incidents/2026-11-16-tests-leak-evolve-ledgers-under-go.md`: the "Open" row. It gets a link to this plan and a note that the CI wrap stays with `ci-test-integration-treewalk-parity`.
- No change to `internal-deliverable.md`, `internal-core.md` or `go/internal/repocontract`: `Claimer` and `LaneMenuFn` stay as they are, and D1 has no pin.

## Red tests

Each test must fail on the parent commit. Prove it by a run without the change. No test changes the working directory: each passes an absolute temporary root, and asserts that nothing appears under it or under the package directory. `chdirForTest` (`cmd_evolve_silent_pins_test.go:24`) is in `package main`, and copies exist in three other packages, so this plan adds no new copy.

| Test | Package | Proves | Acceptance |
|---|---|---|---|
| `TestProjectRootOf_InvertsEvolveDirOf` | `paths` | the inverse holds for any root, and returns `false` for a path not ending in `.evolve` | 1 |
| `TestResolved_Rules` (one row each) | `inboxmover` | one row for each case: off-shape inbox, `.evolve/queue`, relative inbox, two planes, trailing `/.` under a worktree (`errCycleWorktreeInbox`), foreign `LedgerPath`, empty inbox | 2 |
| `TestFacades_RefuseAnOffShapeInboxAndWriteNothing` | `inboxmover` | each facade of the first three F1.6 rows that has an error return gives `errNoProjectRoot`, with a nil `Ledger`. The temporary root and the package directory have no new path. Today the same call writes `CWD/.evolve/ledger.jsonl`. | 2 |
| `TestScopeFacades_RefuseWithAnError` | `inboxmover` | `ResolveContinuationForScope` and `ReconcileConsumedBindings` return `errNoProjectRoot`, and read and delete nothing | 2 |
| `TestFacades_ARootInboxMismatchRefuses` | `inboxmover` | the worktree-root and plane-inbox case and the two-plane case give `errRootInboxMismatch` | 2 |
| `TestFacadeTable_CoversTheExportedAPI` | `inboxmover` | each exported function that takes an `Options` is in a table (D5) | 2 |
| `TestHolderJudge_UsesTheResolvedRoot` | `inboxmover` | `ReleaseStaleClaims` with an off-shape inbox refuses and does not infer `Dir(Dir(InboxDir))` | 2 |
| `TestHostInboxClaimer_RefusesAnInboxOffTheShape` | `cmd/evolve` | `hostInboxClaimer` refuses an inbox that is not `<root>/.evolve/inbox`, and the ledger of the process directory stays empty | 2 |

Existing tests that the change rewrites:

- `TestResolveOpts_AnInboxOutsideAPlaneKeepsTheGivenRoot` (`resolve_root_test.go:38`): the off-shape case now refuses in the first three F1.6 rows.
- `TestMutators_AnExplicitPlaneInboxFromACycleWorktreeIsAllowed`: see D3.
- `TestResolveContinuationForScope_AdoptsAnIDLessClaimByTheLoadersID` (`item_identity_readers_test.go:46`): the test passes `InboxDir: t.TempDir()` with no root. After the change the call returns an error and no continuation. The test names a `<root>/.evolve/inbox` instead.
- `TestConsumeFixItem_ReadsTheInboxOfTheOptions` (`fix_item_test.go:77-84`) puts the root and the inbox in two temporary directories. It names one plane.
- `phases/triage/host_chain_test.go:49-50` sets both fields. It names one plane.
- Every other test of the "Draft-run list".

## Draft-run list

The draft run (`go test -count=1 -p 4 ./...` from `go/`) found these tests. Each one now names one plane.

- `inboxmover`: `TestMutators_AnExplicitPlaneInboxFromACycleWorktreeIsAllowed` (rewritten as `TestMutators_ThePlaneRootWithTheCycleWorktreeAsInboxIsRefusedAsAMixedTree`, D3), `TestCycleWorktreeRefusal_ARelativeStateLinkResolvesThePlane` (the message names `--project-root`), `TestResolveContinuationForScope_AdoptsAnIDLessClaimByTheLoadersID` and `TestConsumeFixItem_ReadsTheInboxOfTheOptions`.
- `cmd/evolve`: `TestScopeHold_AdoptableByTakesAPendingItemAndHeldByDoesNot`, `TestPausedPlan_AScopeThePausedCycleHoldsItselfIsPutFirst` and `TestPausedPlan_AScopeThatAnotherCycleHoldsIsNotPutFirstAndWarns`. Their fixtures used a temporary directory as the evolve dir. They now use `<temp>/.evolve`.
- `core`: `TestProvisionedWorktree_ItsTrackedInboxCopyRefusesTheMover` (the message names `--project-root`).
- `practicewatch`: `TestWatch_FilesIntoTheInboxDirectoryTheOptionsName` set a root and a foreign inbox. It now names the plane of the foreign inbox.
- Compile breaks from the new error return of the two scope facades. They hit the tests of `inboxmover` (through `mustResolveScope` and `mustReconcile`) and `core/continuation_adopt_inbox_test.go`. They also hit the ACS predicates `acs/regression/cycle1515`, `acs/cycle1507` and `acs/cycle1781`. The tagged tier holds them. The ACS predicate `acs/cycle308/reproduce_cycle308_test.go` now passes `Options{ProjectRoot: t.TempDir()}`. It still proves that the release API exists.

## Review findings and answers

| Finding | Answer |
|---|---|
| Round 1: pre-resolving facades | The caller does not pre-resolve. The one resolver `resolved()` (D2) is the single place that derives a root. |
| Round 1: continuation-registry writers | `ReleaseContinuationBindingFor` and the others go through `resolved()` (D2, F1.6). The silent skip is deleted. |
| Round 1: `Roots.ProjectRoot` beside `EvolveDir` | Rejected as approach D. D1 derives one from the other in one place. |
| Round 1: two `Claimer` callers | D4 removes the signature change. The `Claimer` stays, and `hostInboxClaimer` refuses an off-shape inbox through D2. |
| Round 1: syntax-only guard | D5 and the red tests: behavioural, red today. The one syntax scan only checks that the table is complete. |
| Round 1: allow list cannot be pinned | D7 and "CI half": the mechanism has no allow list. |
| Round 2: wrong target set, two inputs, D6 duplicate, green red tests, `Claimer` callers, `ProjectRootOf` claim, D3 placement | Answered in the round 2 text of this plan and kept: F1.3, F1.6, D1, D2, D7. |
| Round 2 (second review), HIGH: D2 and D5 do not match the facade signatures | Verified at `continuation_resolve.go:72`, `continuation_release.go:117`, `holder.go:44`. F1.6 is now one table. Two facades gain an error return. `ClassifyHolder` has no production caller and stays a reader. |
| Round 2 (second review), HIGH: D4 is redundant | Verified. The `Claimer` type has the same Go type. D4 is removed and replaced by a refusal test. |
| Round 2 (second review), HIGH: D3 has no code home | Verified: the Files list had no flag code. The flag-level check is cut from this item to a follow-up. The resolver is the check that every path crosses (F1.7a). "Signals" names the signal of each caller that discards the error. |
| Round 2 (second review), MEDIUM: resolver rule not specified | Verified at `planeRootOf` (`inboxmover.go:80-87`). D2 gives six steps and the red test gives one row for each case. |
| Round 2 (second review), MEDIUM: blast radius understated | Verified. D6 adds a draft run, and the approach table no longer states a count. |
| Round 2 (second review), MEDIUM: D5 guard covers only hand-listed facades | Partly accepted. The completeness test of D5 checks the table against the exported functions. The private `resolvedOptions` type is a follow-up, because the item does not need a type change. |
| Round 2 (second review), MEDIUM: CI half needs a decision | D7 names the owner and the mechanism, and fixes the `go.yml` line numbers. |
| Round 2 (second review), LOW: D1 count pin | Removed. |
| Round 2 (second review), LOW: `chdirForTest` copies | The tests do not change the working directory. |
| Round 2 (second review), LOW: docs and follow-up items | Files lists spec line 19. The messages name `--project-root`. The protected-surface row is corrected. Follow-up items are listed below. |

## Rollout

Enforce at once, in one PR. A refusal has no shadow value: a wrong root writes a wrong row, so a shadow run keeps the fault.

- **Operator behavior change.** A group (a) call with an inbox that is not `<root>/.evolve/inbox`, or with a root that disagrees with the inbox, now refuses. A verb that passes `--evolve-dir` with a base name other than `.evolve` fails at its first mover call, not at flag parsing.
- **Signals.** No new signal kind is needed. Each caller that discards the error names its existing output. `loopwave/plan.go:98-103` logs `[loop] WARN: wave plan: a stale claim stays held` at every wave, and `ClaimLaneScope` logs the `claim-lane-scope` WARN (`outcome.go:150-152`). `continuationForScope` (`cmd_cycle.go:733`) returns no continuation and prints the `[inbox] WARN` line with the error. `ReconcileConsumedBindings` (`cmd_inbox_consume.go:207`) prints `[inbox] WARN: binding release` with the error.
- **Error text.** Each error text names `--project-root`. A refusal at a loop caller is quiet beyond the WARN, so the follow-up check below is the early guard.
- **Evidence.** The red-then-green run of the new tests, `make test` green on Linux with Go 1.23, and a green `cover changed`. No feature flag, no environment value.

## Follow-up items

File each at the same boundary as the PR.

- `every-evolve-dir-inversion-uses-projectrootof`: the other hand inversions of F1.5, with the exact site list.
- `inbox-mover-and-curator-root-source`: `evolve inbox mover` and the curator take their root from `EVOLVE_PROJECT_ROOT` or the working directory (`inboxcmd/cmd_inbox_mover.go`, `cmd_inbox_curator.go:114`). Both pass an explicit root into the facades, so they get the refusal now.
- `cmd-evolve-dir-agrees-with-project-root`: one `paths.CheckEvolveDir(root, evolveDir)` in the cmd layer. It folds `fleetLaneEvolveDirOK` (`cmd_cycle_fleet_evolvedir.go:11`) into it. `loop`, `cycle run` and the wave verbs call it when they parse flags, before any phase. It needs `flag.Visit`, because `--project-root` defaults to `.`. Its red test is `evolve cycle run --project-root A --evolve-dir B/x` exits non-zero before any phase runs. This is optional for this item, because the resolver already refuses the mismatch.
- `inboxmover-rooted-options-type`: a package-private type that only `resolved()` returns, so that the compiler forces each root-dependent path through the resolver.

## Risks

- A caller that relied on the working directory now gets an error. Mitigation: the draft run (D6) finds each test, and the red tests find each facade. Accepted limit: a later production caller is caught only by the refusal at run time and the completeness test.
- A worktree-based caller that set `InboxDir` to the plane inbox now refuses. Mitigation: the draft run lists each caller, and each moves to the plane root in the same PR.
- The follow-up items can lag. The resolver is the backstop until they land.

## Limits

This design does not change `lifecycle`. It changes the cycle-worktree rule in spec lines 17 and 19, as D3 says. It does not change `go/Makefile` or `go.yml`. No outside source applies: the gap is internal, so no web search ran.
