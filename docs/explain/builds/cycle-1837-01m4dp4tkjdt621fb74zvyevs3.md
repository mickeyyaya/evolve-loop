# Build Explanation — Cycle 1837

## Build Binding
- Cycle: 1837
- Base SHA: 441fbf16bd4d0c137c6d24caf58a48ed49f3534e

## Summary
The inbox verbs no longer print a loaded item as skipped. `inboxbatch.LoadFile` now names its two rewrites apart: `sanitized control characters in <fields>` and `truncated overlength <fields> to bound (item loaded)`. `evolve inbox batches|list|rank|show` and `evolve inbox quarantine list` print `WARN skipped` only for an unreadable record and a bare `WARN` for a notice about a loaded one. The CI-red escalation in `ciwatch` now formats its file name with `inboxbatch.FilenameStampLayout`, so the stamp layout has one spelling.

## Rationale
On 2026-09-30, 44 of 241 inbox items printed as `WARN skipped` while all 241 were batched, so the operator read a dropped queue. The loader already had the right classification: `ScanDir` marks a `LoadWarning` as `Unreadable` only when the item is missing. The two verbs read `LoadDir`, the text projection that drops that flag, and put `skipped` in front of every warning. Reading `ScanDir` in the verbs reuses that existing classification, so no new kind field or exported symbol was needed. The notice text had folded control-character stripping and truncation into one phrase, so it could not say `truncated`. `ciwatch` wrote the stamp with its own copy of the layout literal, which `FiledAt` parses back. If either copy changed, `FiledAt` would silently return zero for those items.

## Changed Areas
- `go/internal/inboxbatch/item.go` — `sanitizeItem` records changed, stripped and truncated fields in a `fieldRewrites` value, and `LoadFile` builds its notice from it. `SanitizedFields` still returns the changed fields in the order they were first rewritten.
- `go/internal/inboxbatch/loadfile_test.go` — a table test pins the exact notice for truncation only, for control characters only and for both.
- `go/cmd/evolve/cmd_inbox.go` — `loadPendingInbox` reads `ScanDir`. The new `printLoadWarnings` prints `skipped` only for an unreadable warning.
- `go/cmd/evolve/cmd_inbox_quarantine.go` — `inbox quarantine list` reads `ScanDir` and prints through the same helper.
- `go/internal/ciwatch/ciwatch.go` — the escalation file name is formatted with `inboxbatch.FilenameStampLayout`, not a bare literal.
- `docs/architecture/packages/internal-inboxbatch.md` — documents the two notice kinds, how the verbs print them, and ciwatch as a writer of the stamp.
- `go/acs/cycle1837/predicates_test.go` — the TDD phase's acceptance predicates 001-005 for both inbox items. The build did not edit this file.
- `.evolve/evals/inbox-loader-truncation-reads-as-skipped.md` — the TDD phase's eval for the loader-warning item.
- `.evolve/evals/inbox-filename-stamp-single-source.md` — the TDD phase's eval for the stamp single-source item.

## Design Decisions
`LoadDir`'s `[]string` contract stays as it is because 6 other non-test call sites still use it. The two verbs that print warnings move to `ScanDir`, which `LoadDir` already wraps, so the items and their order are unchanged. Wording stays in `cmd/evolve`, and the classification stays in `inboxbatch` (`LoadWarning.Unreadable`). A duplicate-id warning is about a loaded item, so it now prints as a bare `WARN` too. A control-character-only notice still begins with `sanitized control characters`, which keeps the Task Contract note that `internal/core` renders. The ciwatch fix is a one-token change, because `ciwatch` already imports `inboxbatch`.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1837` passes 5/5 predicates, including both verb subtests of 001-003.
- `go test -count=1` passes for `./internal/inboxbatch`, `./internal/ciwatch`, `./internal/inboxmover/...` and the inbox tests of `./cmd/evolve`.
- `TestLoadFile_NoticeNamesTruncationApartFromControlCharacters` failed before the change, on the old combined notice, and passes after it.

## Compatibility
The items and the exit codes are unchanged. Only the stderr wording changes. A tool that grepped `WARN skipped` to count rewritten items now sees `WARN` and `truncated` or `sanitized`, which describes what actually happened. The ciwatch escalation file name is byte-identical (`TestC1837_005`).

## Limitations
The checked-in inbox still has over-length items that print a truncation notice. Shortening them is the deferred third criterion of `inbox-loader-truncation-reads-as-skipped`. `LoadDir`'s text projection still drops the unreadable flag for its other callers.
