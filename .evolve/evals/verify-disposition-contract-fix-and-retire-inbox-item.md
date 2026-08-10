---
score_cap:
  - criterion: "The literal defect-dispositions.json example in agents/evolve-auditor.md is accepted by the production reader and agrees with the architecture doc"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestAuditorPrompt' ./internal/phases/audit"
  - criterion: "Evidence-shape tolerance (string OR array) holds AND every malformed shape still fails closed"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestClassify_DispositionEvidence' ./internal/phases/audit"
  - criterion: "The defect-disposition-contract-unsatisfiable item is consumed with a durable record naming the contract it retires"
    max_if_missing: 6
    evidence: "grep -rl defect-disposition-contract-unsatisfiable .evolve/inbox/consumed/ >/dev/null"
  - criterion: "The consumed record's verification block cites a merged commit of the two-part fix (PR #422 / #426)"
    max_if_missing: 5
    evidence: "grep -rlE '5f405e92|59579452' .evolve/inbox/consumed/ >/dev/null && git merge-base --is-ancestor 59579452 HEAD"
---

# Eval: verify-disposition-contract-fix-and-retire-inbox-item

> Pins the terminal step of the `defect-disposition-contract-unsatisfiable`
> halt/fix/verify chain. Cycles 1397/1399/1400 FAILed on a contract the agent
> could not communicate rather than on work quality: `agents/evolve-auditor.md`
> described `<workspace>/defect-dispositions.json` in PROSE only, so the
> authoring agent guessed `evidence` as a JSON array against a `string`-typed
> struct field — `json: cannot unmarshal array into Go struct field` — and the
> audit could not grade claims it could not read. PR #422 (`5f405e92`) and PR
> #426 (`59579452`) landed the three-part fix: a LITERAL legal example in the
> auditor persona single-sourced with
> `docs/architecture/continuation-defect-ledger.md`, a tolerant string-OR-array
> evidence unmarshal, and fail-closed rejection of every other shape. This eval
> keeps the fix honest after the item is consumed: the first two caps re-prove
> the runtime behavior on every future cycle — including the negatives, because
> tolerance without fail-closed rejection is a reader that grades nothing — and
> the last two keep the *verification*, not merely the merge, in the durable
> audit trail so a forensics sweep never has to re-derive whether #422/#426 were
> actually verified live. Source incident: cycles 1397/1399/1400 (item filed
> 2026-08-09T15:55:00Z); verification cycle 1420.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| copyable-example | Persona example is readable by the gate's own reader and matches the arch doc | 9/10 | `go test -run '^TestAuditorPrompt' ./internal/phases/audit` |
| tolerance-with-negatives | String and array evidence accepted; unresolvable/empty/object/mixed/null/whitespace still block | 8/10 | `go test -run '^TestClassify_DispositionEvidence' ./internal/phases/audit` |
| incident-consumed | Consumed record exists and names the retired contract | 6/10 | `grep -rl defect-disposition-contract-unsatisfiable .evolve/inbox/consumed/` |
| evidence-cites-merge | Verification cites a fix commit and it is an ancestor of HEAD | 5/10 | `grep -rlE '5f405e92\|59579452' .evolve/inbox/consumed/ && git merge-base --is-ancestor 59579452 HEAD` |
