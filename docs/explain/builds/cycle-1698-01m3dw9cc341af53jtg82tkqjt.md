# Build Explanation — Cycle 1698

## Build Binding
- Cycle: 1698
- Base SHA: 1b0469f626ba8fec795cb5cc53d6181e7e9edc4d

## Summary
Three packages each read the auditor row out of `.evolve/ledger.jsonl` on their own: ship's audit binding, the
composition snapshot in `cmd/evolve`, and the release preflight. They now share one leaf package,
`internal/auditledger`. It owns the row schema, the row identity (`kind=agent_subprocess` and `role=auditor`), the
run-scope rule, and a typed miss sentinel, `ErrNoAuditorForRun`. Each consumer maps the helper's outcomes onto its own
error vocabulary.

## Rationale
The three readers had drifted apart:
- Ship and composition each carried their own copy of the run-scope rule. The cross-run fail-open fix had to be
  applied to each copy separately.
- The release preflight matched `"role":"auditor"` as a raw substring. It missed whitespace-formatted rows and bound
  any row that mentioned the auditor role, whatever its kind.

With one reader, a later fix to the row definition or to run scoping lands once.

## Changed Areas
- `go/internal/auditledger/auditledger.go` — new leaf package. It exports:
  - the scanner `AuditorRows`: one backward scan that returns auditor rows newest first, skips alien lines, and wraps read
    errors with `%w`, so an absent ledger stays `fs.ErrNotExist`.
  - the binder `BindRun`: the single run-scope rule. With a run id it binds only that run's row. With no run id it binds the
    latest row. A miss wraps `ErrNoAuditorForRun` and names the newest refused row's run id and `git_head`.
  - the entry point `LatestAuditorEntry`: `AuditorRows` followed by `BindRun`.
  - the row type `Entry` and the miss sentinel `ErrNoAuditorForRun`.
- `go/internal/auditledger/apicover_named_test.go` — names and executes every export. Covers row identity,
  newest-first order, field decoding, read-error classes, every `BindRun` branch, and the composed entry point.
- `go/.apicover-enforce` — enrolls `./internal/auditledger` in the repo-wide API coverage gate.
- `go/internal/phases/ship/audit.go` — `auditEntry` is now an alias of `auditledger.Entry`. `findLatestAudit` now only
  maps outcomes:
  - the sentinel → `AUDIT_BINDING_NO_AUDITOR`, keeping the refused-run forensics and the "independent review missing"
    wording
  - an absent ledger (`ErrNotExist`) → `AUDIT_BINDING_NO_LEDGER`
  - any other read error → a transient `STATE_IO`
- `go/cmd/evolve/cmd_composition_wiring.go` — deletes the private `auditLedgerEntry` struct and the stale
  `TODO(merge-concurrency-2026)` that asked for exactly this fold. `latestAuditEntry` keeps its composition-only rule:
  rows without a `git_head` cannot seed the snapshot diff, so they are dropped before the shared `BindRun`.
- `go/cmd/evolve/cmd_composition_verdict_guard_test.go` — builds `auditledger.Entry` literals in place of the deleted
  struct. No assertion changed.
- `go/cmd/evolve/cmd_composition_runscope_test.go` — adds pins for the empty-`git_head` skip and for a miss that
  surfaces the shared sentinel.
- `go/internal/releasepreflight/releasepreflight.go` — removes five raw-line regexes and the `bufio` scan.
  `checkRecentAudit` now walks `AuditorRows` and reads typed fields (`ArtifactPath`, `GitHEAD`, `WorktreeTreeSHA`,
  `TS`). Unchanged: phantom skipping, the `SCOPED_OUT`/`NONE` semantics, the missing-ts error, and the lenient
  unparseable-ts rule.
- `go/internal/releasepreflight/releasepreflight_test.go` — adds `"kind":"agent_subprocess"` to auditor fixtures that
  omitted it. Production writes only that shape, so this aligns the fixtures with real rows. No assertion changed.
- `go/internal/releasepreflight/extra_coverage_test.go` — the same fixture alignment: auditor rows gain
  `"kind":"agent_subprocess"`. No assertion changed.
- `go/acs/cycle1698/predicates_test.go` — the TDD phase's acceptance predicates for this task, committed with the
  build.
- `.evolve/evals/unify-auditor-ledger-readers.md` — the TDD phase's eval score caps for this task, committed with the
  build.
- `.evolve/inbox/2026-08-27T06-00-00Z-unify-auditor-ledger-readers.json` — the inbox item that requested this fold,
  removed from the pending queue. The harness claimed it for this cycle, so leaving it pending would let another lane
  pick up the same task. Git records the move as a rename (92% similar) to the consumed path below.
- `.evolve/inbox/consumed/2026-08-27T06-00-00Z-unify-auditor-ledger-readers.json` — the same inbox item at its consumed
  location, rewritten by the harness's consume step. It gains a `consumed` stamp (`at`, `cycle`, `via: ship`), and its
  keys are re-serialized in sorted order. Every original field keeps its value. The move records that this cycle took
  the task and keeps the request on disk as the provenance of the change.

## Design Decisions
- **The helper returns rows, not only the latest row.** The release preflight must walk past phantom rows whose
  artifact was garbage-collected, and composition must skip rows without a head. A single-row API would force each of
  them to rescan the ledger. So the scan (`AuditorRows`) and the scope rule (`BindRun`) are separate exports, and
  `LatestAuditorEntry` composes them.
- **The sentinel is an error value, not a struct type.** Consumers only need `errors.Is`. The forensic detail travels
  in the wrapped message, so no extra error type is exported.
- **Ship keeps its name `auditEntry` as a type alias.** Every existing ship signature and test compiles unchanged.
- **The package stays a leaf.** It imports only the standard library, so it can never cycle with `core`, `ship` or
  `cmd`.
- **`redteamcheck` is deliberately out of scope.** It checks, per cycle, whether every role has reported, and has no
  run scoping.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1698`: 11/11 predicates PASS.
- `APICOVER_PKGS=./internal/auditledger make apicover-enforce`: 5 exported, 5 covered, 100% statement coverage.
- The package suites for `auditledger`, `releasepreflight` and `phases/ship` pass with `-tags integration`.
- The composition tests in `cmd/evolve` pass.
- `go vet ./...` and `gofmt -l .` are clean.

## Compatibility
- Ship's error codes and classes are unchanged. Its no-auditor message is re-worded around the helper's text but keeps
  the refused run id, the `git_head` and "independent review missing".
- The release preflight now ignores auditor-role rows of any other kind. Every production auditor row is
  `kind=agent_subprocess`, so real ledgers bind the same row as before.
- A row whose `ts` or `exit_code` has the wrong JSON type is now skipped as an alien line. Before, it matched the
  substring and failed the ts check. The ledger writer is typed Go, so it does not produce such rows.

## Limitations
- `redteamcheck` still reads the ledger on its own, by design.
- Composition's empty-`git_head` skip stays a consumer-side filter, not a helper option.
