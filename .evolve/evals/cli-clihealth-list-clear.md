---
score_cap:
  - criterion: "clihealth list --json prints only active benches with benched_until"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1787_001' ./acs/cycle1787"
  - criterion: "clihealth clear removes the named bench; an unbenched family exits 1"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1787_00[34]' ./acs/cycle1787"
  - criterion: "clear of an expired bench, which list does not show, exits 1"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1787_020' ./acs/cycle1787"
  - criterion: "the inbox spelling evolve cli-health serves list and clear"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1787_021' ./acs/cycle1787"
---

# Eval: clihealth list/clear verbs

> Operator verbs over clihealth Store.Active/Clear (cycle 1787). Predicates drive the built evolve binary against a temp project root. Audit round 1 of cycle 1787 found two problems. `clear` tested membership against every stored entry, so an expired bench that `list` hides still cleared with exit 0. And the item's own spelling `cli-health` was an unknown command.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| list-active | expired benches excluded | 6/10 | `TestC1787_001` |
| clear-exit | unbenched clear exits 1 | 7/10 | `TestC1787_004` |
| clear-agrees-with-list | expired clear exits 1 | 6/10 | `TestC1787_020` |
| item-spelling | `cli-health` alias | 5/10 | `TestC1787_021` |
