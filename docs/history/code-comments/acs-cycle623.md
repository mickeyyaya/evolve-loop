# Comment history: `acs/cycle623`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle623/predicates_test.go:3` — above `package cycle623`

```text
// Package cycle623 materialises the cycle-623 acceptance criteria for the
// single triage-committed top_n task, token-resolver-production-wiring
// (weight 0.96, inbox
// 2026-07-08T02-10-00Z-token-resolver-production-wiring.json): both
// production composition roots that build gobridge.Deps — internal/adapters/
// bridge.Adapter (NewDefault's engineFactory) and internal/subagent's
// defaultExecAdapter — currently leave Deps.TokenResolver nil (confirmed via
// grep: 0 non-test hits in either file), so token telemetry has been
// silently all-zero since at least cycle 612 (fail-open masks the gap; see
// internal/bridge/engine.go:527's `if e.deps.TokenResolver == nil { return }`
// guard).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…613
// precedent) — each predicate shells `go test -run` over the RED unit tests
// authored this cycle. None is a source-grep; every one exercises the system
// under test (tokenusage.DefaultResolver, Engine.HasTokenResolver,
// Adapter.productionEngineDeps, subagent.execAdapterDeps — each called with
// real arguments, several against real on-disk transcript fixtures) and
// asserts on its result. RED now: internal/tokenusage, internal/bridge,
// internal/adapters/bridge, and internal/subagent all fail to compile
// (DefaultResolver / HasTokenResolver / productionEngineDeps /
// execAdapterDeps all undefined). GREEN once Builder wires
// tokenusage.DefaultResolver(configRoot) into both composition roots per the
// contract documented in each RED test file's header comment.
//
// Scope: the third Acceptance Criteria Summary line ("nil-resolver path
// emits one boot WARN") is dispositioned manual+checklist in
// test-report.md, not predicated here — see that file's Coverage Map for the
// rationale (no reachable nil-resolver case remains once both composition
// roots route through tokenusage.DefaultResolver, which always returns a
// non-nil func) and the Auditor checklist.
```
