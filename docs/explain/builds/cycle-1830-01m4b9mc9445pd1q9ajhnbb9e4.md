# Build Explanation — Cycle 1830

## Build Binding
- Cycle: 1830
- Base SHA: 63eb3af769020bbb87239d4fd42f4536c9603769

## Summary
`evolve phase <name>` and `evolve compose` accept `--cycle N [--project-root P]` and derive the phase request from `.evolve/runs/cycle-N/cycle-state.json` instead of stdin JSON. A cycle that cannot be rerun safely is refused with exit 1 and the reason, and a request on stdin beside `--cycle` is a usage error (exit 10). Without `--cycle` both commands read stdin exactly as before.

## Rationale
Rerunning a phase for a real cycle meant hand-copying the cycle, workspace, worktree, base SHA, run id and goal hash out of the cycle state into stdin JSON, and a typo ran the phase against the wrong workspace. One derivation function in `phasecmd`, called by both commands, keeps the two commands from drifting on which fields they copy or which states they refuse. The rejected alternative was a separate `evolve phase rerun` verb: it would duplicate the dispatch path the phase command already owns.

## Changed Areas
- `go/internal/cli/phasecmd/phase_cycle.go` — new `RequestForCycle`, the one derivation, and `CycleRequestExitCode`, which maps a usage error to 10 and a refusal to 1; it checks usage, then the live run lease, then the state file, then the worktree.
- `go/internal/cli/phasecmd/phase.go` — the phase handler parses `--cycle` and `--project-root` after the phase name (`phaseRequest`) and derives the request when `--cycle` is set; the stdin decode and its exit 11 are unchanged otherwise.
- `go/cmd/evolve/cmd_compose.go` — compose gains the same two flags; `composeRequest` derives the shared request before the dry-run plan prints, so a refused derivation runs no phase.
- `go/internal/cli/phasecmd/phase_cycle_test.go` — names both new exports for the apicover gate: the workspace fallback to the run directory, and the usage-versus-refusal exit mapping.
- `go/acs/cycle1830/helpers_test.go` — TDD phase's predicate fixtures (cycle fixture, refusal table, recording scout, evolve-binary build), unchanged by Build.
- `go/acs/cycle1830/predicates_test.go` — TDD phase's five acceptance predicates, unchanged by Build.
- `go/cmd/evolve/cmd_compose_cycle_test.go` — TDD phase's compose tests proving both phases receive the one derived request and a refusal runs no phase, unchanged by Build.
- `.evolve/evals/cli-phase-cycle-request.md` — TDD phase's eval for the task.
- `docs/operations/runtime-reference.md` — operator-command entries for both flags, the derived fields, every refusal and its exit code, and the unchanged stdin path.
- `docs/architecture/packages/internal-cli-phasecmd.md` — documents `RequestForCycle`, its check order and the exit mapping.
- `docs/architecture/packages/cmd-evolve.md` — the compose entry names the `--cycle` derivation and its pre-dry-run refusal.

## Design Decisions
The lease is checked before the state is read, because a live run may be rewriting that state; a stale or dead-owner lease does not refuse, matching `runlease.LiveOwner`. Usage errors are a private string type so the exit mapping is one `errors.As` rather than a set of sentinels. A terminal on stdin counts as no request, so an operator running the command interactively is not left waiting for EOF. `--cycle 0` keeps the stdin path, as the flag's zero value.

## Verification
The five ACS predicates in `go/acs/cycle1830` (including the real `evolve` binary for compose dry runs), the two `TestCompose_CycleFlag_*` tests and the new `phasecmd` tests pass, as do the phasecmd and cmd/evolve packages with gofmt and go vet clean.

## Compatibility
Callers that pipe a request on stdin without `--cycle` see no change in input, output or exit codes. An unknown flag after the phase name now exits 10 where it was silently ignored before.

## Limitations
Only `cycle-state.json` in the run directory is read; the `run.json` mirror is not used as a fallback. The derivation does not lock the cycle, so a run that starts after the lease check is not detected.
