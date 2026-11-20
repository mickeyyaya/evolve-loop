# Build Explanation — Cycle 2565

## Build Binding
- Cycle: 2565
- Base SHA: 31b6bc80fd0f98c26a512cdd710a923e5c120ded

## Summary
The integration batch now reads the CI verdict of each PR head before it composes the batch. A PR with a red head is ejected with the cause `head ci`.

## Rationale
A red PR head cost a full batch CI run and a bisect (PR #1582). The head verdict is already on GitHub, so the plan reads it with the existing `PeekCI` and spends no new batch CI.

## Changed Areas
- `go/internal/integration/train.go` — adds `CauseHeadCI`, the `HeadCI` port and `checkHeadCI`, which ejects red, no-run and cancelled heads and keeps a pending head out of the batch.
- `go/cmd/evolve/cmd_integration_batch.go` — sets `HeadCI` to `PeekCI` in the production train.
- `docs/architecture/packages/internal-integration.md` — records the head-CI rule.
- `go/internal/integration/train_headci_test.go` — pins the red, no-run, cancelled, pending, gh-failure and green cases.
- `go/cmd/evolve/cmd_integration_headci_test.go` — proves the batch command reaches the check.
- `go/cmd/evolve/cmd_integration_batch_test.go` — extends the fake gh with head runs.
- `go/acs/cycle2565/predicates_test.go` — the five acceptance predicates.
- `.evolve/evals/integration-batch-ejects-a-pr-whose-head-ci-is-red.md` — the eval graders.

## Design Decisions
`HeadCI` is nil-safe, so a train without it plans as before. A gh failure returns an error and never ejects, so a gh outage cannot drop a good PR.

## Verification
The named tests of `go/internal/integration` and `go/cmd/evolve` pass, and the ACS suite is green.

## Compatibility
Green heads plan in the same order with the same kept set.

## Limitations
The check reads the head CI once per plan. A pending head waits for the next batch.
