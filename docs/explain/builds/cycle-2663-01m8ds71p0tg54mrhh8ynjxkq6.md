# Build Explanation — Cycle 2663

## Build Binding
- Cycle: 2663
- Base SHA: 83e996a69ddbd334da16cb1c4278ba09d8a6556b

## Summary
`evolve checkpoint save --all` now skips a linked dev worktree that holds unmerged index entries, saves every other worktree, emits a `CLI_CHECKPOINT_SKIPPED` signal, and exits 0.

## Rationale
`wtcheckpoint.Save` previously called `snapshot` (`git write-tree`) on an unmerged index and failed with exit 128, halting `save --all` and wave boundaries. Checking `git ls-files -u` first lets `Save` return `StatusSkipped` with a reason and no ref. Real save errors still fail with exit 1.

## Changed Areas
- `go/internal/wtcheckpoint/wtcheckpoint.go` — adds `StatusSkipped` (`"skipped"`) to the `Status` constants so callers handle mid-merge skips without string matching errors.
- `go/internal/wtcheckpoint/save.go` — adds `SaveResult.Reason` (`json:"reason,omitempty"`) and `hasUnmerged` (`git ls-files -u`) before `snapshot`, returning `StatusSkipped` with `"mid-merge: unmerged index entries"` and nil error when unmerged entries exist.
- `go/internal/gc/devworktrees_apply.go` — refuses `StatusSkipped` in `saveVerified` so garbage collection never removes a dirty mid-merge dev worktree without a saved checkpoint.
- `go/internal/signalcenter/event.go` — adds `KindCLICheckpointSkipped` (`"cli.checkpoint.skipped"`) to the closed kind set.
- `go/internal/events/publisher/publisher.go` — adds `"checkpoint"` to `publisher.Roles()` for the checkpoint CLI signal root.
- `go/cmd/evolve/cmd_checkpoint_verbs.go` — registers `CLI_CHECKPOINT_SKIPPED`, prints `skipped <worktree> <reason>` in text mode, keeps exit 0 for skipped rows, and emits the Warn signal through `emitCheckpointSkips` and `newRoleSignalCenter`.
- `docs/architecture/event-channels.md` — adds `checkpoint` to the closed role list in section 1 to stay synchronized with `publisher.Roles()`.
- `docs/architecture/packages/internal-wtcheckpoint.md` — documents `StatusSkipped`, the `git ls-files -u` check, and the `internal/gc` refusal guard.
- `docs/architecture/signal-codes.md` — regenerates the generated code table to include `CLI_CHECKPOINT_SKIPPED`.

## Design Decisions
Placing the unmerged index check inside `wtcheckpoint.Save` keeps the skip rule in one home rather than parsing `git write-tree` stderr in the CLI. Checking `git ls-files -u` before `snapshot` avoids copying the index for a worktree that cannot write a tree. `internal/gc` explicitly refuses `StatusSkipped` so cleanup preserves unmerged work.

## Verification
`gofmt -l`, `go vet`, and `ste-lint --strict` pass with zero findings. Unit tests in `wtcheckpoint`, `signalcenter`, `events/publisher`, `gc`, and `cmd/evolve` pass. Integration tests (`-tags integration`) and the cycle 2663 ACS suite verify text output, `--json` envelope rows, `CLI_CHECKPOINT_SKIPPED` signals, real save failures, and post-abort resumption.

## Compatibility
`SaveResult` adds the optional `reason` JSON field with `omitempty`, leaving existing `saved`, `unchanged`, and `clean` payloads unchanged. Single-worktree and `--all` `--json` outputs continue to flow through `cli_contract.go` (`evolve.checkpoint.save/1` envelope).

## Limitations
`Save` checks unmerged index entries with `git ls-files -u`; other mid-operation states without unmerged index entries are unchanged.
