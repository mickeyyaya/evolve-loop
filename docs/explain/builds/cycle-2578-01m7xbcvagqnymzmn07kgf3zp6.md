# Build Explanation — Cycle 2578

## Build Binding
- Cycle: 2578
- Base SHA: 63f36a9357898ca756cd09e7f945ceb628905b29

## Summary
The integration batch now reads the CI verdict of each PR head before it composes the batch. A PR with a red head is ejected with the cause `head ci`.

## Rationale
A red PR head cost a full batch CI run and a bisect. The head verdict is already on GitHub, so the plan reads it with the existing `PeekCI` and spends no new batch CI. This cycle continues cycle 2565, whose push origin rejected after main diverged. The change is the same and runs again on the new base.

## Changed Areas
- `go/internal/integration/train.go` — adds `CauseHeadCI`, the `HeadCI` port and `checkHeadCI`, which ejects red, no-run and cancelled heads and keeps a pending head out of the batch.
- `go/cmd/evolve/cmd_integration_batch.go` — sets `HeadCI` to `PeekCI` in the production train.
- `docs/architecture/packages/internal-integration.md` — records the head-CI rule.
- `go/internal/integration/train_headci_test.go` — pins the red, no-run, cancelled, pending, gh-failure and green cases.
- `go/cmd/evolve/cmd_integration_headci_test.go` — proves the batch command reaches the check.
- `go/cmd/evolve/cmd_integration_batch_test.go` — extends the fake gh with head runs.
- `go/acs/cycle2578/predicates_test.go` — the seven acceptance predicates.
- `.evolve/evals/integration-batch-ejects-a-pr-whose-head-ci-is-red.md` — the eval graders.
- `docs/private/research/archived-2026-11-20/superseded-predicate-packages/cycle2565/predicates_test.go` — the archived predicates of cycle 2565.
- `docs/private/research/archived-2026-11-20/unshipped-build-explanations/cycle-2565-01m7w8xfsbjas1536kgcjmvf5h.md` — the archived explanation of cycle 2565.

## Design Decisions
`HeadCI` is nil-safe, so a train without it plans as before. A gh failure returns an error and never ejects, so a gh outage cannot drop a good PR.

## Verification
The tests of `go/internal/integration` and `go/cmd/evolve` pass on the new base, and the ACS suite for cycle 2578 is run before handoff.

## Compatibility
Green heads plan in the same order with the same kept set.

## Limitations
The check reads the head CI once per plan. A pending head waits for the next batch.
