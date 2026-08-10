---
score_cap:
  - criterion: "agents/evolve-auditor.md carries a fenced JSON disposition example that the production reader (readDispositions) accepts, showing literal values for both a FIXED entry (bare, undecorated cite) and a DEFERRED entry (non-empty reason)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -v -run '^TestAuditorPromptDispositionExampleIsAcceptedByProductionReader$' ./internal/phases/audit | grep -q '^--- PASS: TestAuditorPromptDispositionExampleIsAcceptedByProductionReader'"
  - criterion: "The auditor prompt's example and docs/architecture/continuation-defect-ledger.md's example are the same document when compared as parsed JSON"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestAuditorPromptAndArchDocDispositionExamplesAgree$' ./internal/phases/audit | grep -q '^--- PASS: TestAuditorPromptAndArchDocDispositionExamplesAgree'"
---

# Eval: Literal disposition schema example in the auditor prompt

> `agents/evolve-auditor.md:165` states the contract as
> `{"dispositions":[{"id","status","evidence","reason"}]}` — a list of field
> NAMES. It is not valid JSON and shows no legal value for any field, so the
> agent that must author `<workspace>/defect-dispositions.json` has to invent
> the document shape. Cycles 1397, 1399 and 1400 each invented a different wrong
> one (absent file, absent file, array-typed `evidence`). Source incident: inbox
> `defect-disposition-contract-unsatisfiable`.
>
> The two criteria are deliberately behavioural rather than grep-shaped. The
> first EXTRACTS the fenced example from the prompt and runs it through
> `readDispositions` — the same function the gate calls — so an example that the
> gate would reject cannot be documented as the thing to copy. The second
> compares the prompt's example with the architecture doc's existing one as
> parsed JSON, so reformatting either document is free and drifting one is not
> (`always_full_documentation`; cycle-1342 landed prompt and arch doc together
> for this reason). Doc-sync caps higher than the example itself: two divergent
> examples are two contracts, and the agent obeys whichever it happened to read.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| example-is-legal | prompt example parses via the production reader, both statuses shown with literal values | 4/10 | `go test -run TestAuditorPromptDispositionExampleIsAcceptedByProductionReader` |
| prompt/doc-sync | prompt and arch-doc examples are JSON-equal | 6/10 | `go test -run TestAuditorPromptAndArchDocDispositionExamplesAgree` |
