# 2026-09-26 — a lane was sent work its builder's sandbox forbids, twice

**Class:** pipeline, routing. Two sources describe what a lane may not write, and the routing floor consulted only one of them.
**Surface:** the ADR-0074 routing floor (`inboxbatch.ConsoleRouted` via every routing root) and the builder profile's `sandbox.deny_subpaths`.
**Found by:** the console, diagnosing two consecutive build-floor rejections on the same item.

## What happened

Item `chronicle-s7a-historian-shadow` asks for a new historian agent. Its `files` declare `.evolve/profiles/historian.json (new)`. Triage selected it in cycle 1696 and again in cycle 1699:

1. The TDD phase wrote `TestHistorianProfile_PassesValidateProfile`, which loads the new profile.
2. The builder could not create `.evolve/profiles/historian.json`, because `.evolve/profiles/builder.json` sandboxes it with `deny_subpaths: [..., ".evolve/profiles", ...]`.
3. The build handoff floor refused the handoff: `./cmd/evolve: unit tests FAIL` after 3 corrections.
4. Both cycles sealed FAIL.

The item could never succeed as lane work. The routing floor judged declared paths only with `guards.IsProtectedScope`. That compiled integrity manifest names `builder.json` and `auditor.json`, not the whole directory the sandbox denies. So the item routed to a lane every time.

## Why it was not caught

- The integrity manifest and the sandbox deny list are separate sources for "a lane cannot write this". Nothing required routing to agree with the sandbox.
- The builder cannot see that its failure is structural. It retried the correction ladder until it was exhausted.

## Fix

- `profiles.SandboxConfig.Denies` projects `deny_subpaths` as a path predicate. It reads the same list the OS sandbox enforces.
- `lanerouting.Forbidden(projectRoot, build.ProfileName)` combines protected surface with the build profile's `Denies`.
- `cmd/evolve`'s `laneForbidden` composition root wires that one predicate into the loop's routing roots: the wave seed and widen, the host and CLI claim floors, and `evolve inbox batches`. An AST pin test keeps a bare `guards.IsProtectedScope` out of any `cmd/evolve` root.
- Triage receives the predicate through `triage.Config.LaneForbidden`. Its prompt partition hides the item, and its breaker refuses a `top_n` card that names such a path, even a card that came from the scout rather than the inbox.
- Only the enforced sandbox counts: `sandbox.enabled` must be true, just as the bridge requires.
- Until the fix merged, the item was routed to the console by hand (`route: console-sandbox-denied`).
- Known gaps, filed as a follow-up: `core`'s sequential termination check and the triage registry factory still judge without the predicate. See [internal-lanerouting § Known gaps](../architecture/packages/internal-lanerouting.md#known-gaps).
- The fix was measured on the live queue: 146 → 147 console-routed items. The one new item, `skill-allowlist-for-skill-using-phases`, declares two builder-denied profiles. It was a second latent item no lane could finish.

## Regression coverage

`TestForbidden_RoutesAnItemThatNeedsASandboxDeniedFileToTheConsole`, `TestLaneForbidden_TheRepoBuilderSandboxRoutesAProfileItemToTheConsole`, `TestRoutingRoots_JudgeWithTheLanePredicate` and `TestSandboxConfig_DeniesPathsUnderADeniedSubpath`.
