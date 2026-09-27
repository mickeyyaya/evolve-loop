# Build Explanation — Cycle 1720

## Build Binding
- Cycle: 1720
- Base SHA: 4b3fcead2cad963f19a10bba436283e676d0372e

## Summary
A validated `unified_commitment` now closes all-or-nothing at ship time. When
triage validates a claim that several inbox items share one root cause, the
landing commit carries every member's `consumed/` record or none of them.
Every other inbox id keeps the existing per-item fail-open consumption.
`processUnifiedCommitment`, the triage-side validator, also gains its first
direct unit test, covering five branches.

## Rationale
The ship-time consume loop treated every committed id independently. If member
A was moved and staged and member B then failed (a stage fault, or an
unparseable item that silently resolves as "not found"), the landing commit
closed A alone. That split the unified closeout: the shared fix landed, but
one member stayed pickable and was worked a second time. The smallest correct
fix changes only this seam. The existing loop body becomes a `stage` step that
moves and stages one item. A separate `record` step does the sanctioning,
logging and binding release. The unified path stages every member first and
records them only when all of them closed. Otherwise it rolls back what it
staged.

## Changed Areas
- `go/internal/phases/ship/consume.go` — the loop body is split into
  `itemConsumer.stage` (move + stage, with every existing per-item WARN and
  rollback unchanged) and `itemConsumer.record` (drift sanction, OK log,
  continuation binding release). Three helpers are new. The member reader
  `validatedUnifiedMembers` reads the member set only when
  `unified_projection` sits beside the claim. The unit closer
  `consumeUnified` closes those members as one unit. The undo step
  `rollback` restores the file bytes and re-stages both paths so the index
  matches the restored tree.
- `go/internal/phases/ship/consume_unified_edges_test.go` — pins the two
  builder-chosen member states: a member already in `consumed/` counts as
  closed, and a member outside this ship's committed id set keeps the whole
  unit open.
- `go/internal/phases/triage/unified_test.go` — a table-driven
  `TestProcessUnifiedCommitment` with five subtests: accept-small,
  accept-large, reject-outside-top_n, reject-heterogeneous and reject-spoofed.
  Each one asserts what its branch writes: the projection, the campaign plan,
  the rejection diagnostic and the preserved `top_n`.
- `go/internal/phases/ship/consume_unified_test.go` — TDD-authored fast-tier
  tests for rollback, all-close and scope. Unchanged by the build.
- `go/internal/phases/ship/consume_unified_integration_test.go` — TDD-authored
  real-git test that the landing commit carries all or none of the members.
  Unchanged by the build.
- `go/acs/cycle1720/predicates_test.go` — TDD-authored ACS predicates 001–008.
  Unchanged by the build.
- `.evolve/evals/triage-unified-solution-synthesis.md` — TDD-authored eval
  rows for the cycle predicates. Unchanged by the build.

## Design Decisions
- **The atomic set comes from `unified_projection`, not from the raw claim.**
  Triage deletes the projection when it rejects a claim but keeps the claim for
  forensics. Gating on the claim would therefore make rejected claims atomic.
- **Members close in declared order, after all ordinary ids.** When a member
  fails, no later member is attempted. The members already staged roll back,
  and one `[ship] WARN: unified_commitment member "<id>" ...` names the failure.
- **Rollback re-runs `git add -A -- src dst`** after restoring the files.
  `git reset` would not work here: in a cycle worktree the item may be staged
  but not in HEAD, and a reset would drop it from the landing tree.
- **Sanctioning, OK logs and `releaseBindingForConsume` are deferred to
  `record`.** A rolled-back member therefore leaves the drift-sanction set and
  the continuation registry untouched. Every `internalConsumedPaths` write
  stays in `consume.go`.
- **A member already in `consumed/` counts as closed.** This covers an earlier
  landing that consumed it. Treating it as a failure would leave the commitment
  permanently open.
- **A member that is not in this ship's committed id set fails the unit.** It
  cannot ride this commit, so its siblings must not either.

## Verification
- ACS `go test -tags acs -count=1 ./acs/cycle1720`: 8/8 PASS, including all
  five branch mutants killed by the triage unit test.
- `evolve acs suite --cycle 1720`: verdict PASS, green=180, red=0, skip=53,
  total=233.
- From `go/`, `gofmt -l .` is clean and `go vet ./...` is OK.
  `go test -count=1 ./...` exits 0 with 242 packages ok.
- The ship package's `-tags integration` consume and ship tests pass.
- Both builder edge tests were checked with `-overlay` mutants and each fails
  when its branch is broken.

## Compatibility
- Decisions without `unified_projection` behave exactly as before, which
  covers every existing cycle. Every existing per-item WARN string is kept.
  The 26 pre-existing consume tests that TestC1720_005 runs pass unmodified.
- Both production callers get the change:
  - `shipFromWorktree` for cycle ships, at `go/internal/phases/ship/worktree_ship.go:110`;
  - the direct/manual path in `go/internal/phases/ship/gitops.go:280`.

## Limitations
- On the shipDirect plane path, claimed members live under
  `processing/cycle-N/`, which consume never searches. There they resolve as
  absent, and so the unit rolls back. That matches the earlier per-item
  outcome, where they were never consumed either.
- If the rollback re-stage itself fails, it is logged loudly but cannot be
  retried.
