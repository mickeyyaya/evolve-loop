# Build Explanation — Cycle 2382

## Build Binding
- Cycle: 2382
- Base SHA: 392339f2bab20b676793a244e8b276e17afe5b5b

## Summary
This change adds `evolve inbox trailer-check --range` and the `inbox.curator.trailer.stage` policy key.

## Rationale
Reusing `closesinbox` and `inboxmover.ResolveDispatchState` avoids duplicate trailer parsers and inbox state lookups. Validating `inbox.curator.trailer.stage` inside `InboxCuratorPolicy.validate` avoids a second custom decoder.

## Changed Areas
- `go/cmd/evolve/cmd_inbox_trailercheck.go` — implements `runInboxTrailerCheck`, `checkInboxTrailerRange`, and `parseInboxTrailerCheckArgs` for `evolve inbox trailer-check --range`, failing closed to `enforce` when `policy.Load` returns an error.
- `go/cmd/evolve/cmd_inbox.go` — registers `trailer-check` in `inboxVerbs()`.
- `go/cmd/evolve/registry.go` — adds `trailer-check` to the `inbox` command summary.
- `go/internal/policy/inbox_curator.go` — adds `InboxTrailerPolicy`, `InboxCuratorPolicy.Trailer`, `InboxTrailerStage()`, and strict validation for `shadow` and `enforce`.
- `docs/architecture/cli-contract.md` — documents `inbox trailer-check` and its `evolve.inbox.trailer-check/1` schema.
- `docs/architecture/policy-config.md` — documents `inbox.curator.trailer` and its `shadow` default.
- `docs/architecture/packages/internal-policy.md` — documents `inbox.curator.trailer.stage` and `InboxTrailerStage`.
- `docs/architecture/packages/cmd-evolve.md` — documents `evolve inbox trailer-check`.

## Design Decisions
`Policy.InboxTrailerStage()` reads `Inbox.Curator.Trailer.Stage` directly instead of extending `InboxCuratorConfig`. A new field on `InboxCuratorConfig` breaks `reflect.DeepEqual` assertions in `inbox_curator_test.go`. `docs/architecture/cli-contract.md` documents `inbox trailer-check` outside the core-verb table because `go/internal/agentclass/table.go` is console-owned.

## Verification
All 12 `TestCmd_InboxTrailerCheck` tests, all `internal/policy` tests, and all 7 `cycle2382` ACS predicates pass with 100% statement coverage on `cmd_inbox_trailercheck.go` and `internal/policy/inbox_*.go`.

## Compatibility
Absent `inbox.curator.trailer` keys default to `shadow`, so existing `.evolve/policy.json` files stay valid. Unparseable policy files fail closed to `enforce`.

## Limitations
In a phase cell, `evolve inbox trailer-check` stays class `D` until the console adds it to `go/internal/agentclass/table.go`.
