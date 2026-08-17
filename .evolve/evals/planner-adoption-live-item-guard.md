---
score_cap:
  - criterion: "ResolveContinuationForScope refuses a binding whose scope id is retired with no live pending item, logs the refusal naming the scope, and releases the ghost binding"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1515_003' ./acs/regression/cycle1515 | grep -q -- '--- PASS: TestC1515_003'"
  - criterion: "The guard still resolves a binding whose scope id names a LIVE pending item — refusal is on positive retirement evidence, never on mere absence"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1515_003' ./acs/regression/cycle1515 | grep -q -- '--- PASS: TestC1515_003'"
---

# Eval: planner / adoption live-item guard on the scope-keyed registry read

> `ResolveContinuationForScope` is the ONE seam both the wave planner's lane-scope
> minting and the post-triage adoption path go through, and it used to trust a
> registry hit without asking whether the scope id still named a live pending item —
> so a parked or consumed scope re-armed on every wave with no adoption event and no
> carryover entry (cycles 1487, 1497). This eval pins the read-side belt: refuse and
> release on POSITIVE retirement evidence (the id sits in consumed/, quarantine/,
> processed/, rejected/ or retry/, all outside the batch loader's reach) while still
> resolving live scopes. The second cap is the anti-overreach half and is rated
> higher on purpose: the wave planner also mints lane scopes from carryoverTodos,
> which never have an inbox file, so treating absence as death would trade the
> re-dispatch defect for a salvage-loss defect — every carryover lane's preserved
> work released out from under it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| refuse-retired | Retired scope ⇒ nil, logged, ghost binding released | 8/10 | `TestC1515_003` |
| keep-live | Live scope still resolves (no blanket refusal) | 9/10 | `TestC1515_003` |
