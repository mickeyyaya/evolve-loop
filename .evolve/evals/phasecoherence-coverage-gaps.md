---
score_cap:
  - criterion: "go/internal/phasecoherence statement coverage stays >= 90%"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/phasecoherence/... -coverprofile=/tmp/pc-eval.cover >/dev/null 2>&1 && go tool cover -func=/tmp/pc-eval.cover | awk '/^total:/{gsub(/%/,\"\",$3); exit ($3+0 < 90.0)}'"
  - criterion: "canonicalRole every exact-match switch arm is exercised (func coverage 100%)"
    max_if_missing: 5
    evidence: "cd go && go test ./internal/phasecoherence/... -coverprofile=/tmp/pc-eval.cover >/dev/null 2>&1 && go tool cover -func=/tmp/pc-eval.cover | awk '/[ \\t]canonicalRole[ \\t]/{gsub(/%/,\"\",$NF); exit ($NF+0 < 100.0)}'"
  - criterion: "dispatchNone reject arms exercised (func coverage 100%) — error / unparsable-frontmatter / non-none value all return false"
    max_if_missing: 5
    evidence: "cd go && go test ./internal/phasecoherence/... -coverprofile=/tmp/pc-eval.cover >/dev/null 2>&1 && go tool cover -func=/tmp/pc-eval.cover | awk '/[ \\t]dispatchNone[ \\t]/{gsub(/%/,\"\",$NF); exit ($NF+0 < 100.0)}'"
---

# Eval: phasecoherence coverage gaps (canonicalRole / dispatchNone branches)

> Pins the coverage contract established in cycle 320: the
> `go/internal/phasecoherence` package must keep its statement coverage at or
> above the 90% floor, and the two functions whose dark branches motivated the
> cycle — `canonicalRole` (a pure role-normalization switch) and `dispatchNone`
> (the `dispatch: none` opt-out predicate) — must stay fully exercised. If a
> later cycle deletes the branch tests or regresses the package, these evidence
> commands fail and cap the audit score. Source incident: cycle 320 measured
> canonicalRole at 42.9% (only the default lower-casing arm) and dispatchNone at
> 75.0% (only the happy `dispatch: none → true` arm reached transitively), with
> the package at 85.2% overall.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| coverage-floor | phasecoherence total coverage >= 90% | 6/10 | `go test -cover ./internal/phasecoherence/...` total >= 90.0% |
| canonicalRole-branches | every exact-match switch arm runs (func 100%) | 5/10 | `go tool cover -func` row for canonicalRole == 100% |
| dispatchNone-reject-arms | error / unparsable / non-none arms exercised (func 100%) | 5/10 | `go tool cover -func` row for dispatchNone == 100% |
