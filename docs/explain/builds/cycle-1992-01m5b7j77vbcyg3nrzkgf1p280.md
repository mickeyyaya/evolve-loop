# Build Explanation — Cycle 1992

## Build Binding
- Cycle: 1992
- Base SHA: 2a8186caabc45c83889e1dc6aeb3c172c54f9f41

## Summary
`evolve ship --class manual --pr` now opens the pull request of the pushed branch, or reports the PR that is already open, and prints its number.

## Rationale
The command pushed a branch and left the PR to raw `gh` calls in scripts. Ship stays the single seam: the PR step runs after `shipRunFn` returns OK, so a gh failure can never undo a push.

## Changed Areas
- `go/cmd/evolve/cmd_ship.go` — adds the `--pr` and `--pr-body-file` flags, calls the PR helpers around the ship run, and moves root resolution into `resolveShipRoot` to keep `runShipCmd` inside its size allowance.
- `go/cmd/evolve/cmd_ship_pr.go` — new file with the refusal rules, the branch lookup, the open-PR lookup, `gh pr create` and the result printing.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the new PR tests as serial because they swap `PATH` and `shipRunFn`.

## Design Decisions
Exit 10 refuses a bad request (non-manual class, `--push-only`, body file without `--pr`, or `--pr` on main) before ship or gh runs. Exit 2 reports a gh failure after a successful push. A dry run or a refused ship opens no PR. The open-PR lookup uses `gh pr list --head <branch> --state open`.

## Verification
`go test -count=1 ./cmd/evolve/` passes the eight PR tests and the ratchets. The ACS suite for cycle 1992 reports green=241 red=0.

## Compatibility
Without `--pr`, the flow and exit codes of `evolve ship` are unchanged.

## Limitations
The base branch is fixed to `main`. Only the first open PR is reported.
