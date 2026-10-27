---
score_cap:
  - criterion: "operating-policy.md tells the console to run evolve convergence decide --input <rounds.json> after every review check and to follow the decision"
    max_if_missing: 3
    evidence: "grep -q 'evolve convergence decide --input <rounds.json>' docs/operations/operating-policy.md"
  - criterion: "each review-brief template asks for a full first judgment (J_0) and verify-only later judgments"
    max_if_missing: 4
    evidence: "grep -q 'J_0' docs/operations/operating-policy.md && grep -q 'verify-only' docs/operations/operating-policy.md"
  - criterion: "each review-brief template asks each finding for the kind, blocking, component and late fields"
    max_if_missing: 4
    evidence: "for f in kind blocking component late; do grep -Eq \"(^|[[:space:]\\\"'`{,-])$f[\\\"'`]?[[:space:]]*:\" docs/operations/operating-policy.md || exit 1; done"
  - criterion: "operating-policy.md reports 0 findings under evolve docs ste-lint --strict"
    max_if_missing: 3
    evidence: "./go/evolve docs ste-lint --strict docs/operations/operating-policy.md"
---

# Eval: Convergence V4, console adoption

> Pins the console adoption of the convergence policy (ADR-0126, plan component V4). The operating policy tells the console to run `evolve convergence decide` after each review check. Each review-brief template asks for a full `J_0` and for verify-only later judgments, and it asks each finding for `kind`, `blocking`, `component` and `late`. Source incident: the L2 review chain of 2026-10-07 (seven rounds), which the console handled by an operator memory rule.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| console-runs-decide | the policy names the verb, the timing and the duty to follow the decision | 3/10 | `grep -q 'evolve convergence decide --input <rounds.json>' docs/operations/operating-policy.md` |
| j0-and-verify-only | the brief asks for J_0 and for verify-only later judgments | 4/10 | `grep -q 'J_0' docs/operations/operating-policy.md && grep -q 'verify-only' docs/operations/operating-policy.md` |
| finding-fields | the brief asks for the four typed finding fields | 4/10 | key-pattern loop over `kind blocking component late` |
| ste-clean | the changed doc passes the strict STE lint | 3/10 | `./go/evolve docs ste-lint --strict docs/operations/operating-policy.md` |
