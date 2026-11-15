# Build Explanation — Cycle 2404

## Build Binding
- Cycle: 2404
- Base SHA: 109b1b22f1c889be72db4ec0ff88ee21f2e520d6

## Summary
The inbox curator now proposes a lane reroute only for buildable item kinds. Eligible items must also declare at least one repository file path and have no open pull request.

## Rationale
Unbuildable kinds (`research`, `decision`, `task`), fileless items, and items with an open PR cannot run in a build lane. Storing `RerouteBuildableKinds` in `policy.InboxCuratorConfig` keeps the list in policy data rather than hardcoding it in `Reroutes`. This avoids modifying the protected `.evolve/policy.json` file.

## Changed Areas
- `go/internal/policy/inbox_curator.go` — adds `RerouteBuildableKinds` to `InboxCuratorPolicy` and `InboxCuratorConfig` with the eight-kind default (`bug`, `fix`, `hardening`, `pipeline-repair`, `refactor`, `security`, `techdebt`, `test`) plus non-empty and distinct-name validation.
- `go/internal/policy/inbox_curator_test.go` — verifies default `RerouteBuildableKinds`, JSON override behavior, and validation errors for empty or duplicate lists.
- `go/internal/inboxcurator/curate.go` — adds `RerouteBuildableKinds` and `HasOpenPR` fields to `Inputs` and passes them through `RerouteOptions` to `Reroutes`.
- `go/internal/inboxcurator/reroute.go` — introduces `RerouteOptions` and `canRerouteToLane` so `RouteLaneTarget` proposals require a buildable kind, `len(it.DeclaredPaths()) > 0`, and `!opts.HasOpenPR(it.ID)`.
- `go/internal/inboxcurator/reroute_test.go` — updates `TestReroutes_AGateFileItemOnTheLaneRouteIsADeterministicRerouteToTheConsole` to use a buildable kind and adds `TestReroutes_GatesLaneRerouteOnBuildableKindNamedFilesAndNoOpenPR`.
- `go/internal/inboxcurator/curate_test.go` — sets `RerouteBuildableKinds` in `TestCurate_ALandingRunsTheMatchAndARoutingChangeRunsTheRerouteCheck`.
- `go/cmd/evolve/cmd_inbox_curator.go` — wires `RerouteBuildableKinds` and `openPRMatcher` (exact token boundary match and stderr warning on `gh` or JSON failure) into `curatorRun.curate`.
- `go/cmd/evolve/cmd_inbox_curator_test.go` — updates `TestCmd_InboxCurate_TheRootComesFromTheEnvironmentWhenNotNamed` to use kind `bug` for the rerouted item.
- `go/cmd/evolve/cmd_inbox_curator_openpr_test.go` — tests `openPRMatcherWith` for exact token boundary matching, substring rejection, and stderr warning on `gh` or JSON errors.

## Design Decisions
`Reroutes` takes a `RerouteOptions` struct rather than positional parameters so callers pass `IsProtected`, `BuildableKinds`, and `HasOpenPR` explicitly. `openPRMatcherWith` checks exact token boundaries (`[a-zA-Z0-9_-]`) instead of `strings.Contains` so short IDs like `ad2-7` do not match `ad2-70`. When `gh pr list` or JSON decoding fails, it logs one stderr warning and degrades to `false`.

## Verification
Unit tests in `internal/policy`, `internal/inboxcurator`, and `cmd/evolve`, plus the cycle 2404 ACS predicates `TestC2404_001_AResearchDecisionOrTaskItemOnTheConsoleGetsNoLaneReroute` and `TestC2404_002_AnItemThatNamesNoFilesGetsNoLaneReroute`, pass after all edits.

## Compatibility
Existing `policy.json` files without `reroute_buildable_kinds` continue to parse and inherit the eight-kind default. Callers of `inboxcurator.Curate` that omit `RerouteBuildableKinds` propose no lane reroutes.

## Limitations
`openPRMatcher` runs `gh pr list --limit 1000`. It does not check open pull requests past 1000. When `gh` fails or returns invalid JSON, `openPRMatcher` logs one stderr warning and returns `false`.
