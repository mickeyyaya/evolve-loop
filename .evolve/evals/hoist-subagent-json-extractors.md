---
score_cap:
  - criterion: "No per-call regexp.MustCompile(fmt.Sprintf(...)) remains in production internal/subagent code"
    max_if_missing: 7
    evidence: "test -z \"$(grep -rl 'regexp.MustCompile(fmt.Sprintf' go/internal/subagent --include='*.go' | grep -v _test.go)\""
  - criterion: "internal/subagent behaviour preserved — the package test suite passes"
    max_if_missing: 8
    evidence: "cd go && go test ./internal/subagent/... -count=1"
  - criterion: "The cycle-333 ACS predicates (behavioural extractor gates) pass"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle333/"
---

# Eval: Hoist per-call regexp compiles in internal/subagent

> Pins the cycle-333 code-reduction refactor that replaced three per-call
> `regexp.MustCompile(fmt.Sprintf(...))` JSON field extractors —
> `extractProfileString` (modeltier.go), `extractInt` (ctxadvisory.go), and
> `extractBoolField` (dispatchparallel.go) — with package-level static regexp
> vars plus a thin `matchField` helper. The refactor is behaviour-preserving:
> the only change is that field patterns compile once at package init instead
> of on every call. This eval guards BOTH halves — that the dynamic compile
> stays gone (the code-reduction win) AND that the observable extraction
> behaviour the three exported entry points depend on (ResolveModelTier,
> CheckCtxAdvisory, DispatchParallel) never regresses. Source incident:
> cycle-333 harden/code-reduction goal; the per-call compile was flagged by the
> scout as an avoidable hot-path allocation across 9 literal-arg call sites.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-dynamic-compile | Zero `regexp.MustCompile(fmt.Sprintf` in prod subagent | 7/10 | `grep` over `go/internal/subagent` excl. tests returns nothing |
| behaviour-preserved | `internal/subagent` suite still green | 8/10 | `go test ./internal/subagent/... -count=1` |
| extractor-gates | cycle-333 behavioural predicates pass | 6/10 | `go test -tags acs ./acs/cycle333/` |
