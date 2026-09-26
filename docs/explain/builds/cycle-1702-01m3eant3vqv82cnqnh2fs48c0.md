# Build Explanation — Cycle 1702

## Build Binding
- Cycle: 1702
- Base SHA: b07023d7bd48ba3f191e8940ee8c62a5ff115ede

## Summary
The 2026-06-21 `cmd/evolve` decomposition moved handler files into `go/internal/cli/{opscmd,guardcmd,phasecmd}/`.
Eleven `filepath.Join(root, …)` targets in five historic ACS predicate files still pointed at the old
`go/cmd/evolve/cmd_*.go` paths. The migrations those predicates guard were already complete, yet every one of them
reported a false RED with a bare `no such file or directory`. This build repoints all 11 joins at the files' current
locations. It also makes every `acsassert` path reader that can report a failure append a "file may have moved" hint
when, and only when, the path does not exist. Those readers either take a `TB` or return an `error`: `FileExists`,
`FileContains`, `FileNotContains`, `FileMatchesRegex`, `JSONFieldEquals` and `CountInGoFunc`. The three value-only
readers cannot carry a message without an API change. They are queued as a follow-up inbox item.

## Rationale
Repointing restores each predicate's original assertion on the file it was written to guard. Deleting or skipping the
predicates would clear the RED by weakening the test. The inbox allowed that branch, but triage committed this lane to
the repoint. The hint belongs in `acsassert` rather than in each predicate, because one change there covers every
predicate that fails through a message-carrying reader. That does not cover every predicate. `FileContainsAny`,
`CountOccurrencesAny` and `LineContainsAll` return only a bool or an int, and 46 predicate files under `go/acs` call at
least one of them. On a missing path those readers return false or 0. The predicate then fails with its own content
message, which does not mention a moved file. Changing those signatures would touch every caller, so this build files
that work as a follow-up instead of widening the lane. The hint keys on `errors.Is(err, fs.ErrNotExist)`, so a content
mismatch or a non-ENOENT read error (for example, reading a directory) keeps its plain message. A misleading "moved"
hint would be worse than none.

## Changed Areas
- `go/pkg/acsassert/assertions.go` — adds the unexported `movedHint(err)`. `FileExists`, `FileContains`,
  `FileNotContains`, `FileMatchesRegex` and `JSONFieldEquals` append its output to their read-error message.
  `CountInGoFunc` appends it to its read error, and that error still wraps the OS error with `%w`, so
  `errors.Is(err, fs.ErrNotExist)` holds. The path and the original OS error stay in every message.
- `go/pkg/acsassert/assertions_test.go` — adds a formatting recorder `fmtT` and two tests. The first requires the five
  `TB` readers to report a missing path with the path, the OS error and the hint, and requires a directory read to fail
  without the hint. The second requires `CountInGoFunc`'s missing-path error to carry the path and the hint, to keep
  `fs.ErrNotExist` in its chain, and to have no hint on a directory read.
- `go/acs/cycle50/predicates_test.go` — `TestC50B_002`/`TestC50B_005` now read `go/internal/cli/opscmd/release_preflight.go`.
- `go/acs/cycle47/predicates_test.go` — `TestC47A_003`/`TestC47A_005` now read `opscmd/release_pipeline.go` and its `_test.go`.
- `go/acs/cycle48/predicates_test.go` — `TestC48B_002`/`003`/`004` now read `guardcmd/guard.go` and `guardcmd/guard_test.go`.
- `go/acs/cycle11/predicates_test.go` — `TestC11_004`/`TestC11_005` now read `phasecmd/phase_observer.go` and `phasecmd/phase_watchdog.go`.
- `go/acs/cycle31/predicates_test.go` — two rows of `TestC31_004` now read `guardcmd/postedit_validate.go` and `guardcmd/commit_prefix_gate.go`.
- `go/acs/cycle1702/predicates_test.go` — the TDD phase's acceptance predicates for this cycle, committed with the build.
  They cover:
  - the cycle 50 suite being green;
  - the exact repoint targets;
  - the sibling predicates passing through the real runner;
  - the hint and its negative axis;
  - an AST sweep of every cycle and regression predicate file, with a self-test;
  - an AST census of `acsassert`'s path readers, with every message-carrying reader probed for the hint;
  - a check that the follow-up inbox item names every value-only reader.
- `.evolve/evals/acs-cycle50-predicates-point-at-a-moved-file.md` — the TDD phase's eval (score caps) for this task.
- `.evolve/inbox/2026-09-26T08-55-00Z-acsassert-silent-readers-moved-hint.json` — a new follow-up inbox item,
  `acsassert-silent-readers-cannot-name-a-moved-file`. It declares `go/pkg/acsassert` and names `FileContainsAny`,
  `CountOccurrencesAny` and `LineContainsAll` in its acceptance. The gap these readers leave is queued rather than
  dropped.
- `.evolve/inbox/2026-09-12T05-30-00Z-acs-cycle50-dead-predicate.json` — removed from the open inbox because this
  cycle's lane consumed the item. The pipeline's ship step moved it, not a Builder edit. The item's acceptance
  criteria are unchanged, and this build is graded against them.
- `.evolve/inbox/consumed/2026-09-12T05-30-00Z-acs-cycle50-dead-predicate.json` — the same inbox record in the
  consumed archive. The harness re-sorted its keys and added a `consumed` stamp (`via: ship`), so later triage stops
  offering an item this build resolves while the problem, fix and acceptance text are kept for audit.

## Design Decisions
Only the path arguments changed. Each predicate's assertion, substring and failure text are unchanged, so every
predicate still checks what its author intended. The hint is appended to the existing message
(`<helper>(<path>): <os error> (the file may have moved …)`) instead of replacing it, so log greps on the OS error keep
working. `CountInGoFunc` appends the hint after its `%w` wrap, so callers that test `errors.Is(err, fs.ErrNotExist)`
see no change. `movedHint` stays unexported: it is an internal message detail, not API. The value-only readers keep
their lenient signatures, because some callers rely on "missing means false" and a new form needs a per-caller
decision.

## Verification
- `go test -count=1 -tags acs -v ./acs/cycle1702`: 10/10 PASS. On the bound tree the sweep covered 463 predicate files
  and 227 Go-source targets, with 0 missing.
- `TestC1702_008` and `TestC1702_010` were RED before the `CountInGoFunc`/`JSONFieldEquals` hint and the follow-up
  inbox item existed. Both are GREEN after them.
- `go test -count=1 ./pkg/acsassert`: ok.
- `go test -count=1 -tags acs` on cycles 31, 47, 48 and 50: ok. Cycle 11: every repointed predicate passes.
- `go test -count=1 ./...`: rc=0, 237 packages ok.
- `evolve acs suite --cycle 1702`: green=176 red=0 skip=58 total=234.

## Compatibility
No exported API changed. The only observable difference is a longer failure message for a missing path. For
`CountInGoFunc` the difference is only in `Error()` text. The wrapped error chain is unchanged.

## Limitations
The moved-file hint does not reach predicates that fail through `FileContainsAny`, `CountOccurrencesAny` or
`LineContainsAll`. Those readers return false or 0 on a missing path, so the predicate reports only a content failure.
Inbox item `acsassert-silent-readers-cannot-name-a-moved-file` tracks adding message-carrying forms. `CountInGoFunc`
callers that discard the error, or that print their own message without it, also lose the hint.

The sweep only covers repo-root joins to `.go` files. Deliberately absent targets (retired scripts asserted as deleted,
runtime state, globs) are out of scope.

`TestC11_007` in cycle 11 still fails for an unrelated reason: `ObserverConfig()` moved from
`internal/policy/policy.go` to `internal/policy/dispatch.go`. That is symbol drift in an existing file, not a moved
path, so the hint does not apply to it, and it is left for a follow-up.
