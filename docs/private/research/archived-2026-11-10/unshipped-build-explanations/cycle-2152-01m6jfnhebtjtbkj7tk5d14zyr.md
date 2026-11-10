# Build Explanation — Cycle 2152

## Build Binding
- Cycle: 2152
- Base SHA: 7182679648fff02109a9792da2069b832fe0c95c

## Summary
At the enforce stage the closeout now writes each retrospective pair into `.evolve/retrospectives-pending`. The boundary publish commits each pair into `docs/retrospectives`. The shadow stage is unchanged.

## Rationale
The closeout runs in a lane worktree, so a direct write into `docs/retrospectives` cannot reach main. The dossiers already use a pending directory and a boundary publish. This change reuses that pattern.

## Changed Areas
- `go/internal/recoveryretro/shadow.go` — splits `WriteShadow` so that `WriteInto` writes a pair into any directory.
- `go/internal/recoveryretro/publish.go` — adds `PendingDir` and `PublishPending`, which commit each pair and keep the create-only rule.
- `go/internal/recoveryretro/publish_test.go` — covers the publish paths: commit, held document, half pair, link, commit fault and a git failure.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the new fault-injection test as serial.
- `go/internal/recoveryretro/apicover_named_test.go` — names the new exports in real assertions.
- `go/internal/core/retrospective_shadow.go` — selects the pending directory at enforce and the workspace at shadow.
- `go/cmd/evolve/cmd_loop_dossiers.go` — publishes the pending retrospectives at the boundary under the existing lock.
- `go/cmd/evolve/cmd_dossier.go` — `evolve dossier publish` also publishes the retrospectives.
- `go/internal/core/retrospective_enforce_test.go` — tests the enforce route and the WARN on a write failure.
- `go/cmd/evolve/cmd_loop_dossiers_retro_test.go` — tests that the boundary reaches the publish.
- `go/acs/cycle2152/predicates_test.go` — the acceptance predicates of the cycle.
- `.evolve/evals/recovery-retrospective-c4-stage-key-publish.md` — the eval graders of the task.
- `docs/architecture/packages/internal-recoveryretro.md` — records the two homes and the publish.

## Design Decisions
A pair that has a file in `docs/retrospectives` is dropped from the pending directory and never replaced. A half pair or a link stays pending with a WARN line. A failed commit removes the written files and leaves the pair pending.

## Verification
The five cycle predicates, the core enforce tests and the boundary tests in `cmd/evolve` pass.

## Compatibility
The default stage is unchanged, and shadow writes only the workspace.

## Limitations
The publish makes one commit for each pair. It does not push.
