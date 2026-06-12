---
score_cap:
  - criterion: "Floor count is declaration-primary: ReadDeclaredFloors + CommittedFloorCount source the triage-decision.json companion's committed_floors[] exactly"
    max_if_missing: 8
    evidence: "cd go && go test -run '^TestCountFromDeclaration$' -v ./internal/triagecap/ 2>&1 | grep -q '^--- PASS: TestCountFromDeclaration'"
  - criterion: "Missing / undeclared / malformed companion falls back to the prose counter (fail-open, backward compatible)"
    max_if_missing: 7
    evidence: "cd go && go test -run '^TestCountFallbackToProse$' -v ./internal/triagecap/ 2>&1 | grep -q '^--- PASS: TestCountFallbackToProse'"
  - criterion: "Prose/declaration divergence yields a satisfiable correction string (names the divergent package, references committed_floors), never a reject"
    max_if_missing: 7
    evidence: "cd go && go test -run '^TestFloorDivergenceCorrective$' -v ./internal/triagecap/ 2>&1 | grep -q '^--- PASS: TestFloorDivergenceCorrective'"
  - criterion: "The capacity-clamp reviewer counts declared floors, not prose (cycle-283 prose=12 with honest decl=2 approves; lean prose with over-declared companion rejects)"
    max_if_missing: 8
    evidence: "cd go && go test -run '^TestReviewer_UsesDeclaredFloors$' -v ./internal/triagecap/ 2>&1 | grep -q '^--- PASS: TestReviewer_UsesDeclaredFloors'"
  - criterion: "The throughput-window recorder calibrates K from declared floors, not prose (fixes K-poisoning, cycle 298 H2)"
    max_if_missing: 7
    evidence: "cd go && go test -run '^TestRecorder_DeclaredFloors$' -v ./internal/triagecap/ 2>&1 | grep -q '^--- PASS: TestRecorder_DeclaredFloors'"
  - criterion: "committed_floors is a governed contract surface: documented in the handoff schema and instructed in the triage persona"
    max_if_missing: 6
    evidence: "grep -q committed_floors schemas/handoff/triage-decision.schema.json && grep -q committed_floors agents/evolve-triage.md"
---

# Eval: Declarative floor counter (declaration-primary triage floor counting)

> Pins the ADR-0046 Layer 1 structural fix: `internal/triagecap` floor counting
> moves from prose-regex-primary to DECLARATION-primary, sourcing the committed
> floor count from the `triage-decision.json` companion's `committed_floors[]`
> array (agent-owned ground truth) and retaining the prose counter only as a
> fallback. Source incident: the phantom-floor class that failed cycles 301 and
> 302 (`docs/operations/incident-2026-06-12-triagecap-phantom-floors.md`) — the
> bullet contract's mandated `evidence=`/`source=scout` tokens and coverage prose
> collided with real package basenames, inflating an honest 2-floor commitment to
> 6 and making the capacity-clamp correction directive unsatisfiable, burning both
> corrections and the cycle. Declarations cannot be inflated by contract metadata,
> so they retire the class entirely. This eval keeps the declaration path
> load-bearing: if a future change reverts the reviewer/recorder to prose counting,
> drops the readers, or un-governs the `committed_floors` field, the listed
> evidence breaks and the audit score is capped.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| declaration-count-exact | ReadDeclaredFloors / CommittedFloorCount are declaration-primary | 8/10 | `go test -run '^TestCountFromDeclaration$'` PASS |
| prose-fallback | missing/undeclared/malformed companion → prose fallback (fail-open) | 7/10 | `go test -run '^TestCountFallbackToProse$'` PASS |
| satisfiable-correction | divergence → satisfiable correction, never a reject | 7/10 | `go test -run '^TestFloorDivergenceCorrective$'` PASS |
| reviewer-declared | capacity clamp uses declared floors in both directions | 8/10 | `go test -run '^TestReviewer_UsesDeclaredFloors$'` PASS |
| recorder-declared | throughput window calibrates K from declared floors | 7/10 | `go test -run '^TestRecorder_DeclaredFloors$'` PASS |
| governed-contract-surface | committed_floors documented in schema + persona | 6/10 | `grep committed_floors` schema ∧ persona |
