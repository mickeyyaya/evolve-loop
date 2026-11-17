# Admission splits the coarse cmd/evolve cell: assumptions and evidence

Every number in the options and the recommendation cites an entry here: `A<n>` for an assumption, `E<n>` for evidence. Checked against the worktree at base `ed3bf7d91` on 2026-11-17.

## Assumptions

- **A1** Admission must stay a pure function of known facts. Source: operator-stated (inbox item, plan K5 and ADR-0139 design).
- **A2** An item names files, not symbols, before its build. Source: `go/internal/admission/admission.go:240-253` (text and file matching).
- **A3** The integration train compile catches a shared-symbol break between two merged lanes. Source: `evolve-pr N: integrate` commits in `git log`.

## Evidence

- **E1** The conflict rule is `sharedEdit || meets(x.edits, y.reach) || meets(y.edits, x.reach)`. Citation: `go/internal/admission/admission.go:153-159`.
- **E2** `cmd/evolve` is a leaf and adds no reach. Citation: `go/internal/admission/modulegraph.go:62-64`, `go/internal/admission/admission.go:197-207`.
- **E3** 19 to 22 waiting items named `cmd/evolve` in wave 177. Citation: `docs/research/console-build-or-route-2026-11.md` F1.8.
- **E4** `go/cmd/evolve` holds 1000 files. Citation: `ls go/cmd/evolve | wc -l`, 2026-11-17.
- **E5** Edits in 30 days: `parallel_serial_tests_test.go` 91, `cmd_cycle.go` 53, `registry.go` 39. Citation: `git log --since=30.days --name-only -- go/cmd/evolve`.
- **E6** Each new verb adds one row to `var commands`. Citation: `go/cmd/evolve/registry.go:22`.
- **E7** The policy key `fleet.orchestrator.admission` selects the rule of the whole fleet: `enforce` uses `sharedEdit`, `shadow` uses `conflict`, and both call `sharedEdit`. Citation: `go/internal/policy/fleet_orchestrator.go:154-156`, `go/internal/admission/admission.go:98-101,153-159`.
