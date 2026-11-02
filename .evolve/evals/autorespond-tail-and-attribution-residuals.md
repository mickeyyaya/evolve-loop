---
score_cap:
  - criterion: "A tail-scoped rule's pending send clears as prompt_cleared once the dialog scrolls out of its tail window"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestAutoRespondTail_ScrolledUpDialogClearsPendingForATailScopedRule|TestAutoRespondTail_DialogStillInsideTheTailStaysPending)$' ./internal/bridge"
  - criterion: "The decide path and the resolve path agree on the tail window at every boundary"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestAutoRespondTail_DecideAndResolveAgreeAtEveryTailBoundary$' ./internal/bridge"
  - criterion: "A rule without tail_lines still matches the whole pane when it resolves a pending send"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestAutoRespondTail_RuleWithoutATailKeepsMatchingTheWholePane$' ./internal/bridge"
  - criterion: "The human-input sent-keys line names the fired rule and the auto-respond line keeps naming it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestAutoRespondAttribution_HumanInputSentKeysLineNamesTheRule|TestAutoRespondAttribution_AutoPathStillNamesTheRule)$' ./internal/bridge"
---

# Eval: Auto-responder tail window on the pending path and rule attribution on the human-input path

> Pins residuals 1 and 2 of inbox item `autorespond-tail-and-attribution-residuals`
> (cycle 2101). `resolvePending` matched the full pane while `decideAutoRespond`
> matched only the rule's tail, so a tail-scoped rule under-reported
> `prompt_cleared` and skewed the outcome evidence that the auto-enforce
> promotion sweep reads. The `--human-input` path logged `sent keys` without
> the rule name. Residuals 3 (plan-dialog drift alarm) and 4 (self-matching
> manifest, plan-file harvest) are out of scope for this cycle.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| tail-clears-pending | Dialog outside the tail yields prompt_cleared; dialog inside the tail stays pending | 6/10 | `go test -run TestAutoRespondTail_ScrolledUp...` |
| decide-resolve-parity | Both paths agree at tail boundaries 1, 2, 3, 5 | 7/10 | `go test -run TestAutoRespondTail_DecideAndResolveAgree...` |
| untailed-preserved | An untailed rule keeps whole-pane matching | 6/10 | `go test -run TestAutoRespondTail_RuleWithoutATail...` |
| human-attribution | The human-input line carries `rule=<name>` | 6/10 | `go test -run TestAutoRespondAttribution_...` |
