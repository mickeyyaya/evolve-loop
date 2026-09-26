# internal/lanerouting

Links: [ADR-0074](../adr/0074-typed-signal-contracts-routing-authority.md) (routing authority), [internal-inboxbatch](internal-inboxbatch.md) (`ConsoleRouted`, `PartitionConsole`), [internal-guards](internal-guards.md), [internal-profiles](internal-profiles.md), [incident 2026-09-26](../../incidents/2026-09-26-lane-sent-work-its-sandbox-denies.md).

## Purpose

`lanerouting` answers one question for the routing roots: can a fleet lane ever change this path? `Forbidden(projectRoot, writerProfile)` returns the predicate the ADR-0074 routing floor judges with. An item that declares a forbidden path, or mentions one in its text, is console-owned.

## Design

- **Two sources, one predicate (Specification pattern).** A path is forbidden when it is protected integrity surface (`guards.IsProtectedScope`, the compiled manifest), or when the build profile's **enforced** sandbox denies it (`profiles.SandboxConfig.Denies` over `sandbox.deny_subpaths`). `anyOf` composes the two.
- **Routing follows what is enforced.** `Denies` reads the same `deny_subpaths` list that the bridge turns into OS sandbox rules (`internal/bridge/launch.go`), and only when `sandbox.enabled` is true, since that is the only case where the bridge enforces it. A profile with no sandbox block adds nothing. `Denies` compares cleaned path text; the bridge also resolves symlinks, so a symlinked spelling can differ.
- **The build profile is the writer, on purpose.** Only `tdd` and `build` have `writes_source: true`, and `build` writes the item's declared files.
  - Adding the TDD profile's denials would be wrong. It also denies `agents/` and `skills/`, which is routine builder work, so every persona and skill item would go to the console.
  - Using only paths both profiles deny would repeat the incident: TDD can write `.evolve/evals`, which the builder cannot.
  - The runner loads `<ProjectRoot>/.evolve/profiles/<trim(AgentPromptName)>.json` (`runner/preparation.go`), the same file routing reads. `build.ProfileName` names it once.
- **Composition root.** In `cmd/evolve`, `laneForbidden` is the only non-test use of `guards.IsProtectedScope`.
  - It loads the profile on the first routing question (`sync.Once`), so a root that never routes stays silent.
  - A profile that will not load leaves protected surface only and says so on stderr (`[routing] WARN …`). Failing closed would forbid everything and stop the queue. A builder profile that will not load already fails the build itself.
  - Triage receives the predicate through `triage.Config.LaneForbidden`, both for its prompt partition and for its breaker on `top_n` cards. A nil predicate keeps the old behavior: protected surface for the partition and manifest membership for the breaker.
- **One term.** The routing reasons still read "protected fix surface: <path>". That now means lane-forbidden: on the integrity manifest, or denied by the build sandbox. The manifest alone does not list a sandbox-denied path.

## Invariants

- A sandbox-denied path routes its item to the console. Pinned by `TestForbidden_RoutesAnItemThatNeedsASandboxDeniedFileToTheConsole`, and by `TestLaneForbidden_TheRepoBuilderSandboxRoutesAProfileItemToTheConsole`, which uses the repo's real builder profile.
- Routing follows the profile and its enabled flag, not a copy. Pinned by `TestForbidden_FollowsTheProfileNotACopy`, `TestForbidden_ADisabledSandboxDeniesNothingBecauseNothingEnforcesIt` and `TestForbidden_AProfileWithoutASandboxAddsNothingToProtectedSurface`.
- A missing profile or an empty root is an error, and the composition root degrades loudly, exactly once. Pinned by `TestForbidden_WithoutTheProfileIsAnError`, `TestForbidden_AnEmptyProjectRootIsAnError`, `TestLaneForbidden_WithoutTheBuilderProfileWarnsAndJudgesProtectedSurface` and `TestLaneForbidden_ARootThatNeverRoutesStaysSilent`.
- Every `cmd/evolve` routing root uses the one predicate, and the production `triage.Config` sets it. Pinned by `TestRoutingRoots_JudgeWithTheLanePredicate`, an AST scan.
- Triage's breaker refuses a `top_n` card that names a lane-forbidden path, whatever the card's source. Pinned by `TestTriageClassify_RefusesATopNCardNamingAPathTheLanePredicateForbids`.
- `Denies` matches a denied subpath or anything under it on path-segment boundaries (`.evolve/profilesX` is not `.evolve/profiles`), after `path.Clean`. A directory spelling (`dir/`) that contains a denied subpath is also denied. Pinned by `TestSandboxConfig_DeniesPathsUnderADeniedSubpath`.

## Known gaps

- `core/triage_termination.go`'s claimable-work check still calls `PartitionConsole(items, nil)`. In a sequential cycle whose only pending item is lane-forbidden, it can call that item claimable after triage correctly commits nothing. Filed as inbox item `termination-check-uses-no-routing-predicate`.
- The triage phase-registry factory (used by `serve-phase` and `evolve phase triage`) builds triage with no predicate, and falls back to protected surface without a warning. It is filed in the same item.

## Findings

- Cycles 1696 and 1699 both failed at the build floor on item `chronicle-s7a-historian-shadow`. It declared `.evolve/profiles/historian.json (new)`, which the builder's sandbox denies. See [the incident](../../incidents/2026-09-26-lane-sent-work-its-sandbox-denies.md).
- Measured on the live queue on 2026-09-26 with `evolve inbox batches --json`, before and after the fix: 146 → 147 console-routed items, no item leaving the console. The one new item, `skill-allowlist-for-skill-using-phases`, declares `.evolve/profiles/bug-reproduction.json` and `adversarial-review.json`. Both are builder-denied, so it was a second latent item no lane could finish.
