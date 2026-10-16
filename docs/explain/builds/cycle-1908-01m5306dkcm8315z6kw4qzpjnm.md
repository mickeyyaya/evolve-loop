# Build Explanation — Cycle 1908

## Build Binding
- Cycle: 1908
- Base SHA: 2652b4323740c6090c8c2b25c0936e5e3f23f4b0

## Summary
`evolve dossier sweep [--dry-run]` now exists. It gives `dossier.SweepOrphans`, which had no CLI caller, an operator entry point that recommits orphaned dossier pairs in `knowledge-base/cycles`.

## Rationale
A lane that dies after writing its closeout leaves a `.json` + `.md` pair that no commit tracks. The sweep code existed but nothing could run it. The verb reuses the publish verb's holds (`holdDossierPublish`: plane class, no other live run, git-mutation lock, plane current with origin/main). A second, independent hold set was rejected because it would drift from the publish one.

## Changed Areas
- `go/internal/dossier/sweep.go` — adds `PlanOrphans` and `OrphanPlan`, a non-committing planner; `SweepOrphans` now consumes the same planner, so the dry run and the commit path cannot disagree.
- `go/cmd/evolve/cmd_dossier.go` — adds the `sweep` subcommand (`runDossierSweep`, `sweepDossierPairs`) and names it in the usage and the unknown-subcommand message.
- `go/internal/dossier/anchored_behavior_test.go` — tests `PlanOrphans` for the complete/half split, tree immutability and error wrapping.
- `go/cmd/evolve/cmd_dossier_sweep_test.go` — in-process tests of the verb; the live-lease case now leases under the parent pid, because the caller's own pid is by design not a sibling run.
- `go/acs/cycle1908/predicates_test.go` — end-to-end predicates that run the built binary.
- `.evolve/evals/cli-dossier-sweep.md` — the eval graders for the task.
- `docs/operations/runtime-reference.md` — documents the verb, its holds and its exit codes beside `dossier publish`.

## Design Decisions
Exit codes mirror `dossier publish`: 0 done or nothing to do, 1 held, 2 failure, 10 usage. `--dry-run` takes the probe lock, so it never creates the lock file or blocks a live publisher. The planner is exported because the cmd package is its consumer.

## Verification
Unit tests for the planner, in-process tests for the verb, the ACS predicates `TestC1908_001` to `005`, and the function-size ratchet in `cmd/evolve`.

## Compatibility
No flag, schema or existing exit code changes. `SweepOrphans` keeps its signature and results.

## Limitations
The sweep does not push; the next lane ship pushes the closeout commits. It does not touch dossiers in `.evolve/dossiers-pending`; `dossier publish` owns those.
