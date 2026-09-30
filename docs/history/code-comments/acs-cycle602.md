# Comment history: `acs/cycle602`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle602/predicates_test.go:3` — above `package cycle602`

```text
// Package cycle602 materialises the cycle-602 acceptance criteria for the one
// triage-committed top_n task (see triage-report.md):
//
//   - token-telemetry-s3-engine-launch-instrumentation (inbox 0.93): the
//     Engine.Launch chokepoint (go/internal/bridge/engine.go:335) attributes
//     every LLM invocation's token cost. On each Launch it (1) populates
//     core.BridgeResponse.Tokens from an injected Deps.TokenResolver, (2)
//     appends exactly one record to <Workspace>/llm-calls.ndjson per Launch
//     attempt — so a fallback retry on a different CLI is its own record, making
//     double-dispatch waste measurable — and (3) is fail-open: a resolver error
//     WARNs to Deps.Stderr, leaves resp.Tokens zero, and NEVER turns an
//     otherwise-successful Launch into an error.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…596 precedent).
// Each predicate shells `go test -run` over the named acceptance tests in
// go/internal/bridge/engine_launch_tokens_test.go — every one drives the real
// Engine.Launch pipeline (a live *Engine with an injected TokenResolver + a
// fakeRunner), asserting on resp.Tokens, the on-disk llm-calls.ndjson records,
// and the Launch error path. None is a source grep, so none passes on an
// EMPTY repo: with the feature absent the bridge package fails to compile
// (Deps.TokenResolver / BridgeRequest.Attempt undefined) and every `go test`
// below exits non-zero.
//
// State note: the S3 implementation and these named tests landed in this cycle's
// own goal-hash commit (dca6398c) — this is a resumed run — so the predicates
// are GREEN today (documented as pre-existing GREEN in test-report.md). They
// remain the audit-gating contract: any regression that breaks token attribution
// re-REDs them.
```
