# Build Explanation — Cycle 2136

## Build Binding
- Cycle: 2136
- Base SHA: 940f495ae25f9bbbc621e7fb27ed6625e6465b8b

## Summary
The phase advisor now refuses a launch in fleet mode when the route input has no worktree. Single mode keeps the workspace fallback.

## Rationale
The advisor replaced an empty worktree with the workspace, so the refusal of the bridge never fired in fleet mode. A check in the preflight of the advisor is the smallest change. It runs before the artifact clear, so no side effect happens before the refusal. Two callers did not set the worktree, so both now pass it. A second option, a check in the bridge launcher of core, was rejected because the advisor tests cannot pin it.

## Changed Areas
- `go/internal/core/advisor/launch.go` — adds `fleetWithoutWorktree` and calls it in the preflight, so a blank worktree under fleet mode is refused before any launch.
- `go/internal/core/advisor/launch_test.go` — adds two tests: one for the refusal, one for the worktree thread and the single mode fallback.
- `go/internal/core/advisor/importgraph_test.go` — adds `envchain` and `ipcenv` to the allowed imports of the leaf, because the check reads the fleet key and parses its boolean.
- `go/internal/core/decision_branch.go` — sets `ActiveWorktree` from the cycle state in the retrospective routing, so the advisor accepts the input under fleet mode.
- `go/internal/core/cyclerun_select.go` — adds `decideRoute`, which sets `ActiveWorktree` from the cycle state before the strategy decides, for the same reason.
- `docs/architecture/packages/internal-core-advisor.md` — updates the import count of the leaf to seventeen.
- `docs/architecture/packages/internal-router.md` — replaces the "pinned by no router test yet" note with the test name.
- `go/acs/cycle2136/predicates_test.go` — adds six predicates for the refusal, the thread and the fallback.
- `.evolve/evals/router-active-worktree-fleet-pin-console.md` — adds the eval with the score caps.

## Design Decisions
The check uses `envchain.BoolValue` on `ipcenv.FleetKey`, the same parse as `go/internal/core/orchestrator.go`. A whitespace worktree counts as empty.

## Verification
- `go test -count=1` passes for `./internal/router`, `./internal/core/advisor` and `./internal/core`.
- `go test -tags acs -count=1 ./acs/cycle2136` passes.

## Compatibility
Single mode behavior does not change. In fleet mode, a caller that sets no worktree now gets an error and the router degrades to the static path.

## Limitations
The two new caller lines have no dedicated reachability test. The existing core tests pass with them.
