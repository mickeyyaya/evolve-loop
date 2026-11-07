---
score_cap:
  - criterion: "sync-main refuses and merges nothing while a lane lease is live and the root cycle-state.json is absent"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2208_001' ./acs/cycle2208"
  - criterion: "sync-main still merges when no run lease is live"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2208_002' ./acs/cycle2208"
  - criterion: "the classic root cycle-state.json lease refusal is preserved"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2208_003' ./acs/cycle2208"
---

# Eval: sync-main refuses while fleet lanes are live

> Pins the rule that evolve sync-main scans every run directory for a live lease and does not depend on the root cycle-state.json. Fleet lanes hold their lease under .evolve/runs/ and write no root cycle-state.json, so the old guard let a merge change the tree under a live lane. Source incident: inbox item sync-main-guard-misses-live-lanes, cycle 2208.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| lane-lease-refused | live lane lease and no root cycle-state.json: exit 3, HEAD unchanged, nothing merged | 3/10 | `go test -tags acs -run TestC2208_001 ./acs/cycle2208` |
| no-live-run-merges | stale or absent leases do not block the merge | 5/10 | `go test -tags acs -run TestC2208_002 ./acs/cycle2208` |
| classic-preserved | root cycle-state.json lease still refuses | 5/10 | `go test -tags acs -run TestC2208_003 ./acs/cycle2208` |
