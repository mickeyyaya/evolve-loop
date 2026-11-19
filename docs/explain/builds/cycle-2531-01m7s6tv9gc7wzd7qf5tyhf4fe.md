# Build Explanation — Cycle 2531

## Build Binding
- Cycle: 2531
- Base SHA: b32f7063f2dc92ef18c47353718b941eed47eef2

## Summary
Filing a console-owned inbox item now emits one `INBOX_ITEM_FILED_CONSOLE` INFO signal on the `inbox` channel. `evolve inbox add` also wires a Signal Center, so `evolve events watch --channel inbox --until code=INBOX_ITEM_FILED_CONSOLE` exits when it sees the filing.

## Rationale
Before this change, `Mover.File` returned `ConsoleReason` to the caller without emitting a signal, and `runInboxAdd` passed no Signal Center in `inboxmover.Options`. Emitting directly in `Mover.File` after `publishNewItem` guarantees one signal per successful console-owned filing and zero signals on refused duplicates or lane-dispatchable items.

## Changed Areas
- `docs/architecture/signal-codes.md` — lists `INBOX_ITEM_FILED_CONSOLE` in the `### inbox` registry table.
- `go/cmd/evolve/cmd_inbox_signals_test.go` — updates the `cmd_inbox_add.go` entry in `TestInboxCenterlessRootsArePinned` to record that `Signals` is wired.
- `go/cmd/evolve/cmd_signals.go` — resolves the git repo root in `signalDocRoot` when `EVOLVE_WORKTREE_ROOT` and `EVOLVE_PROJECT_ROOT` are unset, so `evolve signals codes check` finds `docs/architecture/signal-codes.md` from `go/`.
- `go/cmd/evolve/cmd_signals_test.go` — adds `TestRunSignals_FindsRepoDocFromModuleSubdirWhenEnvsUnset` and `TestRunSignals_FallsBackToCwdWhenOutsideGitAndEnvsUnset` (with `assertSignalsCheckInSyncFromDir`) for env-unset runs.
- `go/internal/cli/inboxcmd/cmd_inbox_add.go` — opens the `"inbox"` Signal Center through `host().NewSignalCenter`, defers its close, and passes `Signals: signals` in `inboxmover.Options`.
- `go/internal/cli/inboxcmd/cmd_inbox_add_test.go` — adds `TestCmd_InboxAdd_WiresSignalCenterForConsoleOwnedItems` verifying signal emission for console filings and silence for lane filings.
- `go/internal/inboxmover/lifecycle/apicover_named_test.go` — exercises `CodeItemFiledConsole.BelongsTo(signalcenter.ModuleInbox)` in `TestAPI_EveryExportIsNamed`.
- `go/internal/inboxmover/lifecycle/file.go` — registers `CodeItemFiledConsole` (`INBOX_ITEM_FILED_CONSOLE`) under `ModuleInbox`, derives `class` (`protected` | `kind` | `route`) through `consoleReasonClass`, and emits the INFO event in `emitFiledConsole`.
- `go/internal/inboxmover/lifecycle/file_test.go` — adds unit tests covering protected surfaces, console kinds, authored console routes, lane-dispatchable silence, and refused duplicates.
- `go/internal/inboxmover/lifecycle/mover_test.go` — adds `CodeItemFiledConsole: {"intake"}` to `codeSteps` and updates the inbox code count to 15.

## Design Decisions
- **Registration in `file.go` rather than `lifecycle.go`:** `file.go` owns filing behavior and is not a protected path, leaving `lifecycle.go`, `claim.go`, and `route.go` untouched.
- **Prefix-based `class` derivation:** `consoleReasonClass` maps `inboxbatch.ConsoleRouted` reason prefixes (`protected fix surface:` and `mentions protected path:` -> `protected`; `kind:` -> `kind`). Its default branch classes `route:` and any unknown prefix as `route`.
- **`KindInboxWarning` with `SeverityInfo`:** Matches `inboxcurator.Proposal.Event`, routing to the `inbox` event channel under `module=inbox` without printing spurious stderr lines through `ConsoleSink`.

## Verification
Targeted package tests (`./internal/inboxmover/...`, `./internal/cli/inboxcmd/...`, `./cmd/evolve`) and the native ACS suite (`./go/bin/evolve acs suite --cycle 2531`, including all 6 `go/acs/cycle2531` CLI end-to-end predicates) pass with `red=0`.

## Compatibility
Existing `Mover.File` callers that do not pass `Signals` behave identically, and lane-dispatchable filings emit no new signal.

## Limitations
`evolve inbox add` wires the `"inbox"` Signal Center; any other caller of `inboxmover.File` that leaves `Options.Signals` nil does not publish `INBOX_ITEM_FILED_CONSOLE` events.
