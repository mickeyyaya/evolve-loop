---
score_cap:
  - criterion: "Policy-sourced edge configuration bounds every rendered handoff"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/core/... -run 'Handoff.*(Policy|Edge|Bound)'"
  - criterion: "Invalid or absent edge configuration preserves safe defaults"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/core/... -run 'Handoff.*(Invalid|Missing|Default)'"
---

# Eval: token-opt handoff policy contract

## Criteria

- [code] `cd go && go test -count=1 ./internal/core/... ./internal/policy/... -run 'Handoff.*(Policy|Edge|Bound|Invalid|Missing|Default)'` exits 0.
- [code] A policy fixture with a small cap renders only its declared fields and never exceeds the cap; an undeclared phase edge receives no handoff rather than a raw report fallback.
- [code] A missing handoff block and malformed caps retain the compiled bounded default without panic or unbounded output.
- [model] The implementation has one policy-owned edge vocabulary; it does not duplicate field lists across core helpers.

## Adversarial cases

- Negative: an edge configured with an unknown field is rejected or omitted; it cannot leak arbitrary prior-artifact content.
- Edge/OOD: a zero/negative/oversized cap cannot produce an unbounded handoff.
- Cheapest gaming fake: hard-code the old map while parsing policy but never consume it. The fixture changes policy fields and cap, so that fake fails.
