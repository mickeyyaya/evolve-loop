# Comment history: `acs/cycle987`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle987/predicates_test.go:3` — above `package cycle987`

```text
// Package cycle987 materialises the cycle-987 acceptance criteria for the
// fleet-scoped inbox item `gate-wiring-binding-tests`, which triage split into
// two committed tasks:
//   - ship-gate-stale-attestation-binding-test
//   - qualitygate-reviewer-wiring-binding-test
//
// Both are TEST-ONLY tasks: the deliverable IS a set of DEFAULT-SUITE binding
// tests that catch a severed gate wire. The pre-existing coverage is the exact
// blind spot:
//
//   - internal/phases/ship/commitgate_test.go's stale/missing/valid attestation
//     tests (TestCommitGate_Manual*Attestation_*) sit behind //go:build
//     integration, so they DON'T run in `go test ./...` — a deleted or severed
//     verifyCommitGateAttestation wire is invisible to normal CI.
//   - internal/evalgate/gates_test.go tests qualityGate{}.check() DIRECTLY but
//     never through NewReviewer(...).Review(...), so deleting qualityGate{} from
//     reviewer.go:39's composition slice passes 100% of the existing suite and
//     silently re-admits tautological evals.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-563 precedent). Each
// predicate shells `go test -run '^Name$' -v` over the DEFAULT build suite (NO
// -tags integration) for the binding tests Builder must author, and requires
// that a `--- PASS: <name>` line appears. This genuinely exercises the
// system-under-test (the newly-authored binding tests run against the real
// enforcers):
//
//   - RED now: the named tests do not exist, so no PASS line → predicate fails.
//   - It ALSO fails (correctly) if Builder hides a binding test behind a build
//     tag the default suite skips — the very mistake this task exists to catch.
//   - GREEN only when every binding test exists AND passes in the default suite.
//
// A source-grep (acsassert.FileContains) predicate is deliberately AVOIDED: it
// would pass the moment the magic test name appears in a file, even behind
// //go:build integration — i.e. it could not distinguish the fix from the bug
// (the cycle-85 degenerate-predicate ban).
```
