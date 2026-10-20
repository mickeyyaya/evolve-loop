# internal/lanerouting

Links: [ADR-0074](../adr/0074-typed-signal-contracts-routing-authority.md) (routing authority), [internal-inboxbatch](internal-inboxbatch.md) (`ConsoleRouted`, `PartitionConsole`), [internal-guards](internal-guards.md), [internal-profiles](internal-profiles.md), [incident 2026-09-26](../../incidents/2026-09-26-lane-sent-work-its-sandbox-denies.md).

## Purpose

`lanerouting` answers one question for the routing roots: can a fleet lane ever change this path? `Forbidden(projectRoot, writerProfile)` returns the predicate the ADR-0074 routing floor judges with. An item that declares a forbidden path, or mentions one in its text, is console-owned.

## Design

- **Two sources, one predicate (Specification pattern).** A path is forbidden in one of two cases. In the first case, `guards.IsProtectedScope` judges it protected: it equals a fragment of the compiled manifest or lies inside one. In the second case, the **enforced** sandbox of the build profile denies it (`profiles.SandboxConfig.Denies` over `sandbox.deny_subpaths`). `anyOf` composes the two.
- **A directory that only holds a fragment is lane work.** `go/internal/core` holds 44 fragments and `go/cmd/evolve` holds 3, and both stay lane work ([ADR-0139](../adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md) decision 5). The four guards stop a lane at each protected file. Only `evolve inbox split` judges a directory by containment (`guards.TouchesProtectedSurface`).
- **Routing follows what is enforced.** `Denies` reads the same `deny_subpaths` list that the bridge turns into OS sandbox rules (`internal/bridge/launch.go`), and only when `sandbox.enabled` is true, since that is the only case where the bridge enforces it. A profile with no sandbox block adds nothing. `Denies` compares cleaned path text; the bridge also resolves symlinks, so a symlinked spelling can differ.
- **The build profile is the writer, on purpose.** Only `tdd` and `build` have `writes_source: true`, and `build` writes the item's declared files.
  - Adding the TDD profile's denials would be wrong. It also denies `agents/` and `skills/`, which is routine builder work, so every persona and skill item would go to the console.
  - Using only paths both profiles deny would repeat the incident: TDD can write `.evolve/evals`, which the builder cannot.
  - The runner loads `<ProjectRoot>/.evolve/profiles/<trim(AgentPromptName)>.json` (`runner/preparation.go`), the same file routing reads. `build.ProfileName` names it once.
- **The lazy predicate.** `LazyForbidden(projectRoot, writerProfile, emit)` is the predicate that each routing root uses.
  - It loads the profile on the first routing question (`sync.Once`), so a root that never routes stays silent.
  - A profile that does not load leaves protected surface only (`guards.IsProtectedScope`). A fail-closed predicate forbids everything and stops the queue. A builder profile that does not load already fails the build itself.
  - The fallback emits one registered signal, `INBOX_ROUTING_FALLBACK` (module `inbox`, kind `inbox.warning`, WARN). Its fields name the profile and the project root, and its reason is the load error.
- **Composition roots.** In `cmd/evolve`, `laneForbidden(root, warn)` renders the signal on a writer through `signalcenter.ConsoleSink`. `laneForbiddenTo(root, emit)` sends it to a Signal Center.
  - The cycle root and the simulate root pass their Center (`signals.Emit`). Thus the fallback reaches the cycle's `signals.ndjson`, and the console sink prints it.
  - The other `cmd/evolve` roots (inbox verbs, wave seeding, supply) have no Center, so they print the same coded line on stderr.
  - The triage phase-registry factory (`registryConfig` in `internal/phases/triage`, used by `serve-phase` and `evolve phase triage`) builds the same predicate from `req.ProjectRoot` and `build.ProfileName`. It prints the coded line on stderr.
  - Triage receives the predicate through `triage.Config.LaneForbidden`, both for its prompt partition and for its breaker on `top_n` cards. A nil predicate keeps the old behavior: protected surface for the partition and manifest membership for the breaker. After this change, only a test builds triage with a nil predicate.
- **One term.** The routing reasons still read "protected fix surface: <path>". That now means lane-forbidden: on the integrity manifest, inside it, or denied by the build sandbox. The manifest alone does not list a sandbox-denied path.

## Invariants

- A sandbox-denied path routes its item to the console. Pinned by `TestForbidden_RoutesAnItemThatNeedsASandboxDeniedFileToTheConsole`, and by `TestLaneForbidden_TheRepoBuilderSandboxRoutesAProfileItemToTheConsole`, which uses the repo's real builder profile.
- Routing follows the profile and its enabled flag, not a copy. Pinned by `TestForbidden_FollowsTheProfileNotACopy`, `TestForbidden_ADisabledSandboxDeniesNothingBecauseNothingEnforcesIt` and `TestForbidden_AProfileWithoutASandboxAddsNothingToProtectedSurface`.
- A missing profile or an empty root is an error, and the composition root degrades loudly, exactly once, with the registered code. Pinned by `TestForbidden_WithoutTheProfileIsAnError`, `TestForbidden_AnEmptyProjectRootIsAnError`, `TestLazyForbidden_WithoutTheProfileEmitsTheRegisteredFallbackCodeOnceAndJudgesProtectedSurface`, `TestCodeRoutingFallback_IsRegisteredToTheInboxModule`, `TestLaneForbidden_WithoutTheBuilderProfileWarnsAndJudgesProtectedSurface` and `TestLaneForbidden_ARootThatNeverRoutesStaysSilent`.
- The cycle root sends the fallback to its Signal Center. Pinned by `TestLaneForbiddenTo_EmitsTheRoutingFallbackIntoTheCycleCenter`.
- A directory that only holds protected surface is not forbidden, and a directory inside a fragment is. Pinned by `TestForbidden_ADirectoryThatOnlyHoldsProtectedSurfaceStaysLaneWork` and `TestLaneForbidden_ADirectoryThatOnlyHoldsProtectedSurfaceStaysLaneWork`.
- Every `cmd/evolve` routing root uses the one predicate, and the production `triage.Config` sets it. Pinned by `TestRoutingRoots_JudgeWithTheLanePredicate`, an AST scan.
- The triage registry factory judges with the builder sandbox. Pinned by `TestRegistryFactory_TriageJudgesWithTheBuilderSandboxPredicate` and `TestRegistryConfig_WithoutTheBuilderProfileEmitsTheRoutingFallbackCode`.
- Every orchestrator root in `cmd/evolve` gives the no-work check the lane menu. Only the `routingtest` root can leave it nil. Pinned by `TestCompositionRoots_EveryOrchestratorRootWiresTheLaneMenu` (an AST scan), `TestWireOrchestrator_TheNoWorkCheckAsksTheLaneMenu` and `TestWireSimulateOrchestrator_TheNoWorkCheckAsksTheLaneMenu`.
- A root whose only item declares a protected file offers no lane work. Pinned by `TestLaneMenuOf_TheProductionPredicateWithholdsARootThatHoldsOnlyAProtectedFileItem`. With `TestRunCycle_EmptyTriageOverWorkTheLaneMenuWithholdsIsPlannedNoWork` in `core`, the cycle then ends as planned no-work, not as a claim failure.
- Triage's breaker refuses a `top_n` card that names a lane-forbidden path, whatever the card's source. Pinned by `TestTriageClassify_RefusesATopNCardNamingAPathTheLanePredicateForbids`.
- `Denies` matches a denied subpath or anything under it on path-segment boundaries (`.evolve/profilesX` is not `.evolve/profiles`), after `path.Clean`. A directory spelling (`dir/`) that contains a denied subpath is also denied. Pinned by `TestSandboxConfig_DeniesPathsUnderADeniedSubpath`.

## Known gaps

- The claim floor accepts an item that lists a directory holding a fragment, and the split refuses to put that directory in a lane part. The trade-off keeps four lanes supplied (ADR-0139 amendment).

## Findings

- Cycles 1696 and 1699 both failed at the build floor on item `chronicle-s7a-historian-shadow`. It declared `.evolve/profiles/historian.json (new)`, which the builder's sandbox denies. See [the incident](../../incidents/2026-09-26-lane-sent-work-its-sandbox-denies.md).
- Measured on the live queue on 2026-09-26 with `evolve inbox batches --json`, before and after the fix: 146 → 147 console-routed items, no item leaving the console. The one new item, `skill-allowlist-for-skill-using-phases`, declares `.evolve/profiles/bug-reproduction.json` and `adversarial-review.json`. Both are builder-denied, so it was a second latent item no lane could finish.
