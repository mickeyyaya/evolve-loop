---
score_cap:
  - criterion: "The exit signal of a quota wall names the quota, and an unnamed prompt keeps BRIDGE_EXIT_UNKNOWN_PROMPT"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/bridge/launchoutcome -run 'TestClassify_ARouterUsageLimitExitNamesTheQuotaNotTheUnknownPrompt|TestClassify_AnUnknownPromptExitKeepsTheUnknownPromptSignal'"
  - criterion: "a router launch that exits 85 at the weekly limit ends the cycle quota-paused"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2253_001 ./acs/cycle2253"
  - criterion: "cycle resume --all lists the cycle that paused at the router"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2253_002 ./acs/cycle2253"
  - criterion: "the quota pause signal names the quota and the exit signal is not BRIDGE_EXIT_UNKNOWN_PROMPT"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2253_003 ./acs/cycle2253"
  - criterion: "a non-quota exit 85 at the router still degrades to the static spine"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2253_004 ./acs/cycle2253"
---

# Eval: The signal of a router usage-limit exit names the quota

> Pins the signal contract of a router-dispatch wall. In wave 160 the cycles 2221 and 2222 met the weekly usage
> limit during the router dispatch and sealed SKIPPED_UNKNOWN with the signal BRIDGE_EXIT_UNKNOWN_PROMPT. The
> quota-pause path (acceptance 1 and 2) was a console follow-up in cycle 2246. Cycle 2253 reaches it through the
> unprotected `router_observer.go` and `cyclerun_select.go`, so it carries the caps below.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| signal-names-quota | quota exit signal differs from the unknown prompt | 5/10 | `TestClassify_ARouterUsageLimitExitNamesTheQuotaNotTheUnknownPrompt` |
| router-wall-pauses | exit 85 at the weekly limit gives ErrAllFamiliesExhausted | 4/10 | `go test -tags acs -run TestC2253_001 ./acs/cycle2253` |
| resume-all-lists | the paused cycle is listed by PausedLaneScopes | 5/10 | `go test -tags acs -run TestC2253_002 ./acs/cycle2253` |
| quota-signal | ORCHESTRATOR_QUOTA_PAUSED names the quota | 6/10 | `go test -tags acs -run TestC2253_003 ./acs/cycle2253` |
| no-over-pause | a non-quota exit 85 does not pause | 6/10 | `go test -tags acs -run TestC2253_004 ./acs/cycle2253` |
