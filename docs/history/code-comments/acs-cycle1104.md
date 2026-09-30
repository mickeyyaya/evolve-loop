# Comment history: `acs/cycle1104`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1104/predicates_test.go:3` — above `package cycle1104`

```text
// Package cycle1104 materialises the cycle-1104 acceptance criteria for the
// single triage-committed top_n task of this lane:
// `continuation-nonclaim-scope-binding` (ADR-0076 slice C, G2).
//
// The defect: continuation bindings key ONLY off inbox-claimed processing
// scopes (.evolve/inbox/processing/cycle-N/*.json). Cycle-1078's failing lane
// (`chain-boundary-loop`) took its scope from the wave planner, so there was no
// item file for the FAIL-release path to stamp — the preserved snapshot was
// orphaned and no later attempt could ever adopt it. G2 adds the second
// scope-identity class: lane-scope todo ids (the authoritative
// <workspace>/lane-scope.json pin), reusing internal/continuation.Continuation
// VERBATIM (no forked schema), with claim resolution still tried first so
// G1/PR #363 semantics are untouched.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1102
// precedent). The produce side is an UNEXPORTED method on core.Orchestrator
// (stampContinuationManifest) and the adoption seam is unexported plumbing, so
// neither can be imported from here; each predicate instead shells
// `go test -run` over the RED contract tests authored this cycle in
// go/internal/{continuation,inboxmover,core}. Every one of those exercises the
// system under test — driving the real registry writer/reader, the real
// resolver, the real stamp method over a real git fixture, and RunCycle
// end-to-end through the production resolver closure — and asserts on returned
// values, on-disk artifacts and dispatched phase requests. None is a source-grep
// of production code (the cycle-85 degenerate-predicate ban).
//
// RED now: WriteRegistryEntry / ReadRegistryEntry / RegistryPath /
// ResolveContinuationForScope do not exist and WithContinuationResolver still
// takes the 2-arg closure, so all three packages fail to compile.
```

### `go/acs/cycle1104/predicates_test.go:96` — above `func TestC1104_003_ResolveFallsBackToLaneScopeWithClaimFirst(t *testing.T) {`

```text
// TestC1104_003_ResolveFallsBackToLaneScopeWithClaimFirst — AC1 + AC3, the
// resolve side. Drives ResolveContinuationForScope over real fixtures: a cycle
// with NO processing claim at all resolves its lane-scope binding (the
// cycle-1078 case), a claim-stamped continuation still WINS over the registry
// (G1 untouched), an UNSTAMPED claim does not suppress the fallback, and
// multi-id lanes resolve in declared order.
```
