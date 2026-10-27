# Build Explanation — Cycle 2038

## Build Binding
- Cycle: 2038
- Base SHA: 2831f561e598d137f4068abd56eb6a81fec12f5e

## Summary
The operating policy adopts the convergence policy for console review chains. The review-brief template enforces J_0 and verify-only later judgments with structured fields.

## Rationale
The operating policy gives the operator one direct place to read console rules. An alternative was to put the template in `docs/architecture/convergence-policy.md`, but that separates operator process from operating policy. Running `evolve convergence decide` after each check stops unending review cycles and enforces bounded convergence.

## Changed Areas
- `docs/operations/operating-policy.md` — adds the console decide rule and the review-brief template with J_0, verify-only, and typed finding fields.
- `go/internal/convergence/rules_test.go` — adds unit tests that verify rounds validation for both unresolved and resolved configurations.
- `go/acs/cycle2038/predicates_test.go` — adds cycle acceptance predicates that test operating policy rules, template field requirements, and ste-lint compliance.
- `.evolve/evals/convergence-policy-v4-console-adoption.md` — defines evaluation criteria and verification graders for task acceptance.

## Design Decisions
The canonical review-brief template resides in `docs/operations/operating-policy.md` with explicit fields for severity, kind, component, blocking, and late status.

## Verification
All cycle predicates `TestC2038_001` through `TestC2038_006` pass, `evolve docs ste-lint --strict` reports zero findings, and unit tests in `internal/convergence` pass.

## Compatibility
The rounds decoder and production validator contracts remain unchanged.

## Limitations
This change covers console adoption. Automated pipeline loop integrations follow in subsequent plan components.
