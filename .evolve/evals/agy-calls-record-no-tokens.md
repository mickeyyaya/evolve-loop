---
score_cap:
  - criterion: "The real agy 1.4 pane gives 0 under the agy-tmux manifest token_line_regex, and the conversation store gives a non-zero output count for the same dispatch"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2178_001' ./acs/cycle2178"
  - criterion: "A conversation of another worktree or outside the dispatch window is not counted"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2178_002' ./acs/cycle2178"
  - criterion: "A conversation whose last user input equals the window end to the microsecond is counted, and one microsecond later is not"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2178_003' ./acs/cycle2178"
  - criterion: "A stored time with trimmed trailing zeros that equals the window start is counted, and one microsecond earlier is not"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2178_004|TestC2178_005' ./acs/cycle2178"
  - criterion: "The bridge and tokenusage packages stay green"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/bridge/... ./internal/tokenusage/..."
---

# Eval: agy calls record their tokens

> Pins the agy token source of the inbox item agy-calls-record-no-tokens (cycle 2178). agy 1.4 prints no
> "Thought for X, N tokens" line, so the manifest token_line_regex reads 0 on the real pane. The agy
> conversation collector (commit f1bd9e110) reads the tokens from agy's own store and already works live:
> 137 agy_conversation rows against 92 none rows, and every none row has exit 85 and cause exhausted (quota).
> The residual defect is the text comparison of last_user_input_time against the window bounds. Acceptance 2
> needs one live wave and is deferred by triage.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| pane-vs-store | Manifest regex gives 0, store gives non-zero | 7/10 | `go test -run TestC2178_001` |
| scope-negative | Other worktree and out-of-window rows excluded | 6/10 | `go test -run TestC2178_002` |
| end-edge | Input time equal to the window end is counted | 6/10 | `go test -run TestC2178_003` |
| start-edge | Trimmed-zero stored time equal to the window start is counted | 6/10 | `go test -run 'TestC2178_004|TestC2178_005'` |
| regression | Bridge and tokenusage packages stay green | 7/10 | `go test ./internal/bridge/... ./internal/tokenusage/...` |
