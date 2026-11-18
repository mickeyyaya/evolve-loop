# internal/inboxtest

## Purpose

`internal/inboxtest` owns the inbox test fixtures that the tests of `cmd/evolve` and `internal/cli/inboxcmd` share.

## Design

- The package has one home for each fixture. The two test packages call it instead of keeping a copy each.
- The test-duplicate ratchet (`go/.test-dup-baseline`) counts equal helper bodies. Moving the inbox verbs into `inboxcmd` made a second copy of each fixture. This package removes those copies.
- `CurationRoot` makes an empty inbox, seeds it, and points `EVOLVE_PROJECT_ROOT` at it.
- `WriteLiveRunLease` writes a fresh lease for the running process, so a test sees a live run.

## Invariants

- Each exported fixture takes `testing.TB` and fails the test on an I/O error.
- Every exported symbol is named in a real assertion. Pinned by `TestFixtures_BuildAndReadBackAnInboxTree`, `TestFixtures_EmptyRootHasAnInboxAndPolicyAndRoutedAreWritten` and `TestFixtures_WriteLiveRunLeaseRecordsTheRunningProcess`.

## Findings

None.
