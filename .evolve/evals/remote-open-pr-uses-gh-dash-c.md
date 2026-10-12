---
score_cap:
  - criterion: "remoteOpenPR never passes the unsupported -C flag to gh"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestRemoteOpenPR_NeverPassesDashCToGh ./cmd/evolve"
  - criterion: "remoteOpenPR runs gh inside the repo directory"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestRemoteOpenPR_RunsGhInsideTheRepoDirectory ./cmd/evolve"
  - criterion: "prune keeps a superseded ref with an open PR when gh rejects -C"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags integration -run TestBranchesPrune_AnOpenRemotePRKeepsASupersededRef ./cmd/evolve"
---

# Eval: remoteOpenPR must not pass -C to gh

> The gh CLI has no -C flag, so `evolve branches prune --dry-run=false` failed on any host with a remote and a real gh. The integration stub ignored its arguments and hid the defect. Source: cycle 1858.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-dash-c | gh gets no -C | 4/10 | `go test -run TestRemoteOpenPR_NeverPassesDashCToGh` |
| cwd | gh cwd is the repo | 5/10 | `go test -run TestRemoteOpenPR_RunsGhInsideTheRepoDirectory` |
| integration | stub rejects -C | 6/10 | `go test -tags integration -run TestBranchesPrune_AnOpenRemotePRKeepsASupersededRef` |
