---
score_cap:
  - criterion: "gitexec exports Remote, MainBranch and RemoteMain with RemoteMain equal to origin/main"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2293_001 ./acs/cycle2293"
  - criterion: "No production Go file outside gitexec declares the constant origin/main"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2293_002 ./acs/cycle2293"
  - criterion: "core/resume_base.go and inboxmover/curate.go read gitexec.RemoteMain"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2293_003 ./acs/cycle2293"
  - criterion: "The core and inboxmover tests stay unchanged and pass"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2293_004|TestC2293_005' ./acs/cycle2293"
---

# Eval: One home for the main ref

> Pins the single home of the origin/main ref in gitexec.RemoteMain. Before this
> task, core/resume_base.go and inboxmover/curate.go each held a private
> constant with the same literal. Source: inbox item one-home-for-the-main-ref (cycle 2293).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exported-home | gitexec exports RemoteMain | 6/10 | `go test -tags acs -run TestC2293_001 ./acs/cycle2293` |
| no-copy | no outside constant holds origin/main | 5/10 | `go test -tags acs -run TestC2293_002 ./acs/cycle2293` |
| callers-wired | both callers read gitexec.RemoteMain | 6/10 | `go test -tags acs -run TestC2293_003 ./acs/cycle2293` |
| tests-unchanged | the package tests stay frozen and green | 5/10 | `go test -tags acs -run 'TestC2293_004|TestC2293_005' ./acs/cycle2293` |
