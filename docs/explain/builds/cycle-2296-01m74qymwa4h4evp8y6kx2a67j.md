# Build Explanation — Cycle 2296

## Build Binding
- Cycle: 2296
- Base SHA: 4f1b959d5c804bf598855ab83194f1836e695c54

## Summary
`evolve gc` now sweeps remote branches. At `shadow` it only reports. At `enforce` it deletes a remote branch only after a verified bundle of the tip. The branch needs a merged pull request whose head equals the remote tip.

## Rationale
Decision D10 of the gc plan needs a remote sweep, and component X8 builds it. Decision D4 defines a landed branch. Decision D2 requires a verified bundle before each delete. The sweep reuses the operator salvage layout of the local branch sweep, so one directory holds every bundle. The delete goes through `gh api`, because the ship gate bans `git push`.

## Changed Areas
- `go/internal/gc/remotesweep.go` — the planner and the apply step: tips from `git ls-remote`, the keep rules, the tip re-check, the bundle with `git bundle verify`, then the API delete.
- `go/internal/gc/remotegh.go` — the production `gh` adapter for the pull request read, the ruleset lookup and the branch delete.
- `go/internal/gc/remotesweep_test.go` — tests of every keep reason, every apply failure and the shadow and enforce paths.
- `go/internal/gc/remotegh_test.go` — tests of the `gh` adapter, including its decode errors.
- `go/internal/gcpolicy/gcpolicy.go` — the `remote_sweep.stage` policy key and `RemoteStage`, which returns `enforce` only for the exact word.
- `go/internal/gcpolicy/gcpolicy_test.go` — tests that an absent, unknown or mis-cased stage falls back to `shadow`.
- `go/cmd/evolve/cmd_gc.go` — the `exec` field of `gcRun` and the call of the remote sweep after the other sweeps.
- `go/cmd/evolve/cmd_gc_remote.go` — the `evolve gc` step: a dry run is always `shadow`, and a failed plan is a warning.
- `go/cmd/evolve/cmd_gc_remote_test.go` — tests of the step with a scripted `git` and `gh`, a failed project run and the kill closure of `newGCRun`.
- `go/acs/cycle2296/predicates_test.go` — the predicates of the task, written by the TDD phase.
- `.evolve/evals/gc-legacy-gap-x8-remote-sweep-at-shadow.md` — the eval of the task, written by the TDD phase.
- `docs/architecture/packages/internal-gc.md` — the Amended by line names D10, and a design bullet describes the sweep.
- `CHANGELOG.md` — the release note of the remote sweep.

## Design Decisions
The stage is policy data. A string that is not exactly `enforce` is `shadow`, so a typo cannot enable a delete. A failed pull request read fails the plan, so a lookup failure never reads as "not landed". The apply step fetches the branch and compares the fetched tip with the planned tip, so a push after the plan keeps the branch. A failed bundle or verify keeps the branch and joins into the returned error. I rejected `git push --delete` because the ship gate bans it, and I rejected a delete before the bundle because D2 forbids it.

## Verification
The predicates `TestC2296_001` to `TestC2296_013` all pass. They cover the shadow report and the enforce delete. They also cover the bundle order, the keep reasons and the moved tip. The last ones cover the policy stage, the production `gh` path and the coverage of the changed functions.

## Compatibility
The default stage is `shadow`, so a policy with no `remote_sweep` key deletes nothing. `evolve gc --dry-run` never calls the delete API. A host without `gh` or without the remote gets a warning and the run continues.

## Limitations
The sweep deletes only landed branches. The idle-branch rule of D10 (D7 reasons) is not part of this change. The pull request read is limited to 1000 pull requests, so an older merged branch is kept.
