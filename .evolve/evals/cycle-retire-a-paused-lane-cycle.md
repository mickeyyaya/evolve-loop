---
score_cap:
  - criterion: "A paused cycle that holds a grant is retired by evolve cycle retire and cycle resume --all no longer lists it"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2300_001|TestC2300_002' ./acs/cycle2300"
  - criterion: "The seal of the retire is in the project ledger with the reason and the ledger chain still verifies"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2300_003' ./acs/cycle2300"
  - criterion: "The verb refuses a cycle whose lane is live, an unknown cycle and a missing cycle, and changes nothing on a refusal"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2300_004|TestC2300_005' ./acs/cycle2300"
  - criterion: "The retire releases the inbox claims of the cycle and keeps its worktree for gc"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2300_006' ./acs/cycle2300"
---

# Eval: evolve cycle retire for a paused lane cycle

> Pins the operator verb that retires a paused lane cycle. Before it, cycle reset sealed only the one-directory
> ledger, and cycle resume could grant a resume but never revoke it, so a cycle abandoned by the operator stayed
> in the cycle resume --all list for ever. Source incident: inbox item cycle-retire-a-paused-lane-cycle (cycle 2300).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| grant-revoked | Retired cycle leaves the resume --all list and cannot be granted again | 4/10 | `go test -tags acs -run 'TestC2300_001\|TestC2300_002' ./acs/cycle2300` |
| ledger-seal | Seal with the reason in the project ledger | 5/10 | `go test -tags acs -run 'TestC2300_003' ./acs/cycle2300` |
| refusals | Live lane, unknown cycle, missing cycle refused and nothing changed | 5/10 | `go test -tags acs -run 'TestC2300_004\|TestC2300_005' ./acs/cycle2300` |
| claims-and-worktree | Inbox claims released, worktree kept | 6/10 | `go test -tags acs -run 'TestC2300_006' ./acs/cycle2300` |
