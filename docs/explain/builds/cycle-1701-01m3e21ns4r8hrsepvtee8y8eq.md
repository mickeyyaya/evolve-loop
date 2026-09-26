# Build Explanation — Cycle 1701

## Build Binding
- Cycle: 1701
- Base SHA: b666bf128b7900ef4c637082c8b21687663a8d89

## Summary
Ship's audit-bound tree now has a single source: the auditor ledger entry's `worktree_tree_sha`. The dead fallback that
parsed an `audit_bound_tree_sha:` line out of the auditor's report is deleted, along with its regex. The in-function
comment that described that fallback as live now states the single meaning.

## Rationale
`verifyAuditBinding` filled `opts.internalAuditBoundTreeSHA` from the ledger's `worktree_tree_sha`, or else from the
report comment. The else-branch never reached a success path. An empty ledger tree is refused by `verifyPredicateReceipt`
(`AUDIT_BINDING_MALFORMED`, precondition class), because the evidence identity needs a 40-hex tree. If that check were
bypassed, the treefence check would refuse it too (`AUDIT_BINDING_TREE_MISMATCH`). Neither gate reads the fallback value.
The fallback still gave the field a second documented meaning: the base tree (`HEAD^{tree}`) that the auditor persona
binds. A base tree can never equal a changes-commit tree, and that second meaning nearly led a later slice to wire
`shipDirect` against the wrong comparand. Deleting the fallback removes that meaning from the code.

## Changed Areas
- `go/internal/phases/ship/audit.go` — replaces the `if`/`else if` with one assignment,
  `opts.internalAuditBoundTreeSHA = entry.WorktreeTreeSHA`, and deletes `auditBoundTreeSHARe`. Also rewrites the block
  comment and the function doc so they name the ledger as the only source and explain why the report cannot be one:
  the report binds the unchanged base tree.
- `go/internal/phases/ship/direct_bound_witness_integration_test.go` — comment only. It called the report-comment
  fallback "unreachable". Now that the fallback is deleted, it says the report is never a binding source. No assertion
  changed.
- `go/internal/phases/ship/audit_bound_tree_source_test.go` — the TDD phase's in-package tests, committed with the
  build as an untagged (default-tier) file. They check that:
  - the report comment never binds the tree, even when it names the correct tree
  - an empty ledger tree is refused as Malformed/precondition
  - the ledger tree wins over a conflicting comment
  - no non-test ship source holds the `audit_bound_tree_sha:` literal
- `go/internal/phases/ship/realgit_testhelpers_test.go` — now holds `auditOpts` and `preSeedTOFU`, moved verbatim from
  `audit_gaps_test.go`. This untagged shared-helper file is where helpers go when default-tier tests need them, which
  the new test file does.
- `go/internal/phases/ship/audit_gaps_test.go` — `auditOpts` and `preSeedTOFU` were moved out of it. Its tests and
  their callers are unchanged.
- `go/acs/cycle1701/predicates_test.go` — the TDD phase's acceptance predicates for this task, committed with the build.
- `.evolve/evals/audit-binding-report-comment-fallback-is-dead-code.md` — the TDD phase's eval score caps for this task,
  committed with the build.
- `.evolve/inbox/2026-09-12T09-30-00Z-audit-binding-report-comment-fallback-dead.json` — removed from the pending inbox.
  The bug report it held is the task this cycle fixes, so it is no longer pending work. Leaving it would re-queue a
  fixed defect.
- `.evolve/inbox/consumed/2026-09-12T09-30-00Z-audit-binding-report-comment-fallback-dead.json` — the same record, moved
  to `consumed/` by the inbox consumer. Its keys are re-serialized in sorted order, and a `consumed` stamp
  (`at`, `cycle`, `via: ship`) was added. Its problem, fix, and acceptance text are unchanged, so the audit trail keeps
  the original report next to the proof that it was consumed.

## Design Decisions
- The new tests are untagged, not `integration`-tagged. They need only real git and the shared helpers, both of which
  the default tier already has. A newly added integration-tagged file makes the build floor run the whole ship
  integration tier under a 120s timeout, and that tier takes about 130s on its own. The floor timed out on the tagged
  file even though every test passed. As untagged tests, they run in the default ship suite (about 54s) and also under
  `-tags integration`, where the ACS predicates run them.
- A plain assignment, not a guarded one. An empty `WorktreeTreeSHA` leaves the field empty, which is what an unbound
  audit already means to every consumer. The refusal stays with `verifyPredicateReceipt`, the gate that already owns it,
  so the empty-tree error keeps its existing code and message ("predicate evidence invalid").
- No new early-return guard for an empty ledger tree. It would duplicate an existing refusal and change the
  operator-facing error for no behavioral gain.
- The `audit_bound_tree_sha` key in `ship-binding.json` is untouched. It is a different surface: ship writes that sidecar
  from the ledger binding, and `native.go` reads it back.

## Verification
- `go test -count=1 -tags integration ./internal/phases/ship` passes in 130.1s. The floor's default-tier command,
  `go test -count=1 -timeout 120s ./internal/phases/ship`, passes in 53.7s and includes the 4 new tests.
- `go test -tags acs -count=1 ./acs/cycle1701` passes all 4 predicates. Before the change, predicates 001 and 004
  failed: the report comment bound the tree, and the parser literal was present at `audit.go:167`.
- `gofmt -l .` is clean, and `go vet ./...` is clean.

## Compatibility
- No flow that ships today changes. Every successful ship already bound the ledger tree, because the fallback value was
  refused before it could be used.
- Error codes and messages are unchanged.
- An auditor report that still carries an `audit_bound_tree_sha:` line is simply ignored for binding.

## Limitations
- The auditor persona may still write the `audit_bound_tree_sha:` line into its report. This change does not edit the
  persona. The line is now inert metadata.
- The write-once witness comment on `internalAuditBoundTreeSHA` in `go/internal/phases/ship/native.go` still says the
  field is set "after parsing audit-report.md", and it cites stale line numbers. That file is a protected control-plane
  path (ADR-0064), so a cycle cannot edit it. The correction is left as console work.
