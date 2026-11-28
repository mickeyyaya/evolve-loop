# Build Explanation — Cycle 2687

## Build Binding
- Cycle: 2687
- Base SHA: e703c94329802a40f044bce55452bfa52f19df3a

## Summary
Inbox intake now emits a typed warning (`INBOX_HUB_FILES_UNNAMED`) when an item declares a whole hub directory. This applies to `go/cmd/evolve` and `go/internal/core` in `files`.

## Rationale
Naming a whole hub directory in `files` defeats per-file admission cells and blocks parallel fleet lanes. Emitting a structured shadow warning at intake surfaces the hub name and suggested file family without breaking existing callers.

## Changed Areas
- `go/internal/inboxmover/hub_intake.go` — registers `CodeHubFilesUnnamed` (`INBOX_HUB_FILES_UNNAMED`) under `ModuleInbox`. Its `WarnUnnamedHubFiles` function finds whole hub directories in `item.DeclaredPaths()`, deduplicates them, and emits `inbox.warning` `WARN` events.
- `go/internal/cli/inboxcmd/intake.go` — invokes `inboxmover.WarnUnnamedHubFiles(item, opts.Signals)` in `admitInboxItem` so both `evolve inbox add` and `NewFiler` share the check.
- `go/internal/inboxmover/hub_intake_test.go` — unit tests cover signal registration, hub detection, trailing-slash normalization, deduplication, and non-hub or named-file negatives.
- `go/internal/cli/inboxcmd/cmd_inbox_add_intake_test.go` — verifies `evolve inbox add` prints the typed warning for whole hub directories and emits none for named files.
- `go/internal/cli/inboxcmd/filer_test.go` — verifies `NewFiler` emits `INBOX_HUB_FILES_UNNAMED` through the shared intake path.
- `docs/architecture/signal-codes.md` — regenerated the generated signal code catalog to include `INBOX_HUB_FILES_UNNAMED`.

## Design Decisions
Placing `WarnUnnamedHubFiles` in `go/internal/inboxmover/hub_intake.go` avoids edits to the protected `go/internal/inboxmover/inboxmover.go` and `go/internal/inboxmover/lifecycle/` surfaces. Wiring it in `admitInboxItem` in `go/internal/cli/inboxcmd/intake.go` covers both CLI intake and request-channel filing in one place.

## Verification
Unit tests in `go/internal/inboxmover`, `go/internal/cli/inboxcmd`, and `go/internal/signalcenter` pass with `-count=1` and `-race`. All four `go/acs/cycle2687` predicates (`TestC2687_001` through `TestC2687_004`) pass.

## Compatibility
Shadow-only behavior: items that declare whole hub directories still file with exit status 0 while emitting `inbox.warning WARN INBOX_HUB_FILES_UNNAMED`.

## Limitations
Enforce-mode refusal requires a new stage key in the protected `go/internal/policy/stage_keys*.go` registry and remains a console follow-up.
