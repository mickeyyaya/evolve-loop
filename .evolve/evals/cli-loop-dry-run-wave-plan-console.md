---
score_cap:
  - criterion: "evolve loop --dry-run prints a wave_plan with width, reason and each lane's items with weights"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2115_001' ./acs/cycle2115"
  - criterion: "the dry-run plan equals the lanes the wave engine dispatches for the same inbox"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2115_002' ./acs/cycle2115"
  - criterion: "the dry run writes no claim, lease or cycle state"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2115_003' ./acs/cycle2115"
  - criterion: "sequential fleets and empty inboxes print an empty plan with a reason; undispatchable items are excluded with their reason"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2115_00[456]' ./acs/cycle2115"
  - criterion: "with fleet.budget.stage=enforce the dry-run width and lanes equal what Engine.Size and Dispatch give; shadow holds the count"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2115_00[78]' ./acs/cycle2115"
  - criterion: "the dry-run CLI reaches the budget sizing layer without probing live quota or writing state, and stays silent without a budget block"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2115_0(09|10)' ./acs/cycle2115"
---

# Eval: evolve loop --dry-run prints the wave plan

> Pins the console part of the dry-run wave plan: `printLoopDryRun` printed the
> config only, so the operator could not see which items and lanes the next
> wave would dispatch. Source: inbox item cli-loop-dry-run-wave-plan-console,
> cycle 2113 (predicates re-homed to cycle 2115).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| plan-printed | wave_plan carries width, reason, lane items with weights | 4/10 | `go test -tags acs -run TestC2115_001 ./acs/cycle2115` |
| one-plan-source | dry-run lanes equal the engine dispatch for two fixtures | 4/10 | `go test -tags acs -run TestC2115_002 ./acs/cycle2115` |
| read-only | tree snapshot unchanged, stale claim kept | 3/10 | `go test -tags acs -run TestC2115_003 ./acs/cycle2115` |
| edge-cases | empty plan with reason; unmet-dependency item excluded | 6/10 | `go test -tags acs -run 'TestC2115_00[456]' ./acs/cycle2115` |
| budget-sized-width | enforce shrinks the preview as it shrinks the wave; shadow holds the count (audit M1, cycle 2113 (predicates re-homed to cycle 2115) round 3) | 5/10 | `go test -tags acs -run TestC2115_00[78] ./acs/cycle2115` |
| budget-cli-reach | the CLI dry run reaches `Engine.Size`, never probes, never writes | 5/10 | `go test -tags acs -run TestC2115_0(09\|10) ./acs/cycle2115` |
