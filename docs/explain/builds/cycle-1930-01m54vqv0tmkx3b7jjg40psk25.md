# Build Explanation — Cycle 1930

## Build Binding
- Cycle: 1930
- Base SHA: 2056979ec95cdb16916b65583815222f7c021f60

## Summary
The `./cmd/evolve` unit suite now runs faster. A paired run on the loop host at the same load took 215.1 s on the base and 153.9 s on this build (−28%). A new named package wall budget, `addedtests.PackageWallBudget` (90s), gives the slow-package alarm back. `testlatency -package-budget` applies it, and `make test-budget` runs it on the fast suite.

## Rationale
The build floor moved every go-test run onto `addedtests.PackageTimeout` (20m). That removed the only alarm for a slow package. One named budget at a quarter of the timeout or less catches slow growth long before a deadline. The suite time came mostly from git fixtures built per test, real process scans and real worktree provisioning. Templates and the existing seams remove that cost, and no test is deleted or skipped.

## Changed Areas
- `go/internal/addedtests/wall_budget.go` — adds `PackageWallBudget = "90s"` in its own file, the one named wall budget. `budget.go` holds `PackageTimeout` and stays unchanged, because it is a protected control-plane path (ADR-0064).
- `go/internal/addedtests/budget_test.go` — pins that the budget parses and is at most `PackageTimeout/4`.
- `go/cmd/testlatency/main.go` — adds `-package-budget`, which sets `--budget` from the named value and turns on `--fail-over-budget`; a parse error exits 2.
- `go/cmd/testlatency/budget_test.go` — pins exit 1 over the budget and exit 0 within it.
- `go/Makefile` — adds the `test-budget` target, the production caller of `-package-budget`.
- `go/cmd/evolve/cmd_loop_control.go` — `gcOrphanSessions` honors the existing `gcInjectedReapers` seam from `cmd_gc.go`, so tests do not scan the host's tmux sessions.
- `go/cmd/evolve/cmd_branches_test.go` — builds the branch fixture once per test binary through `gittest.FromTemplate`.
- `go/cmd/evolve/cmd_checkpoint_test.go` — builds the checkpoint hub store from a template.
- `go/cmd/evolve/cmd_gc_devwt_test.go` — builds the dev hub store from a template, fakes the reapers and the process host, and puts a fake `lsof` on PATH.
- `go/cmd/evolve/cmd_gc_processes_test.go` — puts an `lsof` shim scoped to the orphan pid in a temp dir on PATH, so the orphan test does not scan every process.
- `go/cmd/evolve/cmd_loop_gc_worktree_test.go` — builds the project repo from a template and fakes the reapers, the process host and `lsof`.
- `go/cmd/evolve/cmd_loop_m4_test.go` — adds `inPlaceStubWorktree`, fakes the session reaper and keeps the run-dir GC in shadow for the stub loop.
- `go/cmd/evolve/cmd_loop_coverage_test.go` — stub orchestrators use `inPlaceStubWorktree` instead of real git worktrees.
- `go/cmd/evolve/cmd_loop_cyclelevel_test.go` — the same stub worktree provisioner.
- `go/cmd/evolve/cmd_loop_goal_source_test.go` — the same stub worktree provisioner.
- `go/cmd/evolve/cmd_loop_wavesync_test.go` — builds the origin and runtime clones from a template.
- `go/cmd/evolve/cmd_syncmain_test.go` — builds the repo and its bare remote from templates.
- `go/cmd/evolve/cmd_worktree_dev_test.go` — builds the dev hub from a template; `setGitRemoteOriginURL` repoints the copied remote.
- `go/cmd/evolve/cmd_worktree_dev_landed_test.go` — the landed hub uses the template dev hub.
- `go/cmd/evolve/cmd_worktree_test.go` — `initRepo` copies a template repo.
- `docs/architecture/packages/internal-addedtests.md` — records the wall budget and its pin.
- `docs/architecture/packages/cmd-testlatency.md` — records `-package-budget` and its pin.

## Design Decisions
The budget is a Go duration string, the same shape as `PackageTimeout`, so `go test` and `testlatency` parse it alike. `-package-budget` reuses the existing `--budget` and `--fail-over-budget` path, not a second report. A template copy keeps an absolute remote URL from the template dir, so the tests repoint `remote.origin.url` after the copy. Rejected: deleting or skipping the slow tests, because the acceptance forbids a weaker suite. Rejected: `t.Parallel()` on the seam-swapping tests, because they mutate package globals.

## Verification
`go test -tags acs -count=1 ./acs/cycle1930` passes 5/5. `go test -count=1 ./cmd/testlatency ./internal/addedtests` passes. Paired `go test -count=1 ./cmd/evolve`: base 215.1 s, build 153.9 s, both green; a solo build run took 126.0 s while other fleet lanes ran.

## Compatibility
`--budget` and `--fail-over-budget` keep their contract (predicate 004). `PackageTimeout` is unchanged. The production change in `gcOrphanSessions` uses the real reapers when no test injects fakes.

## Limitations
The loop host runs other fleet lanes, so an absolute time below 105 s was not shown on an idle host. `make test-budget` covers `FAST_PKGS` only, and it is not yet a CI gate.
