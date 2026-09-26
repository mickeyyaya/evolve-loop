---
score_cap:
  - criterion: "A complete two-member declaration whose authored files lie outside both members' declared scopes yields the file-scope advisory (block=false) naming both members"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run ^TestTDDScopeGate_TwoMemberFileScopeDriftIsAdvised$ ./internal/topngate"
  - criterion: "A complete two-member declaration whose authored files lie inside either member's scope yields no advisory, and an undeclared scope fails open"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestTDDScopeGate_(TwoMemberInEitherScopeStaysSilent|TwoMemberWithoutDeclaredScopeStaysSilent)$' ./internal/topngate"
  - criterion: "An incomplete multi-member declaration still blocks on scope-mismatch before any file-scope judgement"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run ^TestTDDScopeGate_IncompleteMemberDeclarationBlocksBeforeScopeCheck$ ./internal/topngate"
  - criterion: "Single-member lanes keep today's file-scope advisory text and behavior"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestTDDScopeGate_(SingleMemberFileScopeAdvisoryTextUnchanged|FileScopeDriftIsAdvisory|FileScopeBinding)$' ./internal/topngate"
  - criterion: "The topngate package stays vet-clean and race-green"
    max_if_missing: 5
    evidence: "cd go && go vet ./internal/topngate/... && go test -race -count=1 ./internal/topngate/..."
---

# Eval: Multi-member file-scope advisory

> Pins the construction-level file-scope check for multi-member lanes at the
> TDD->Build boundary. The cycle-1620 salvage (`multi-slug-lane-scope-reconciliation`)
> made `tddScopeGate.check` reconcile a multi-member lane's declared set against
> the committed set and return straight away, so the cycle-1111 advisory, which
> checks authored test files against the committed item's scout `targetFiles`,
> stopped running for any lane with two or more members. The cycle-1620 audit
> (L3) filed this as a follow-up. Reproduced in cycle 1703
> (`.evolve/runs/cycle-1703/bug-reproduction-report.md`): a complete two-member
> declaration authoring outside both members' scopes logs nothing through the
> enforce-stage reviewer.
>
> The fix unions every committed member's declared `targetFiles` and runs one
> ANY-overlap check against that union, advisory only, after a complete
> reconciliation. Single-member text stays byte-identical.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cycle-1620 L3 gap | drift outside every member's scope is advised, never blocked, naming both members | 9/10 | `go test -run TestTDDScopeGate_TwoMemberFileScopeDriftIsAdvised ./internal/topngate` |
| union, not first-member / not all-members | in-scope for EITHER member stays silent; no declared scope fails open | 8/10 | `go test -run 'TestTDDScopeGate_(TwoMemberInEitherScopeStaysSilent\|TwoMemberWithoutDeclaredScopeStaysSilent)' ./internal/topngate` |
| ordering | set mismatch still blocks on its own reason before scope is judged | 8/10 | `go test -run TestTDDScopeGate_IncompleteMemberDeclarationBlocksBeforeScopeCheck ./internal/topngate` |
| single-member invariance | today's advisory text and behavior unchanged | 7/10 | `go test -run 'TestTDDScopeGate_(SingleMemberFileScopeAdvisoryTextUnchanged\|FileScopeDriftIsAdvisory\|FileScopeBinding)' ./internal/topngate` |
| toolchain | package vet-clean and race-green | 5/10 | `go vet ./internal/topngate/... && go test -race ./internal/topngate/...` |
