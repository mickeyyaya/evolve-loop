---
score_cap:
  - criterion: "A run dir whose reports are under the threshold and whose logs and prompts are over it does not insert context-condense"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2141_002|TestC2141_003' ./acs/cycle2141"
  - criterion: "The router repro tests for run_dir.artifact_bytes pass"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestDigest_RunDirArtifactBytes_' ./internal/router"
  - criterion: "Report artifacts over the threshold still insert context-condense"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2141_004' ./acs/cycle2141"
  - criterion: "The phase spec records the run dir replay and the threshold follows from it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2141_006' ./acs/cycle2141"
---

# Eval: context-condense trigger measures report artifacts only

> Pins the fix for `run_dir.artifact_bytes`, which summed every file of the run dir. Logs, prompts and event streams passed 102400 bytes on 131 of 133 replayed run dirs, so the context-condense trigger never decided anything. Source incident: cycle 2110 (`context-condense-report.md`, a no-op WARN) and the review of PR #1105 (M2).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| noise-negative | Prompts, logs and event streams do not fire the trigger | 4/10 | `go test -tags acs -run 'TestC2141_002\|TestC2141_003' ./acs/cycle2141` |
| repro | Router repro tests pass | 5/10 | `go test -run TestDigest_RunDirArtifactBytes_ ./internal/router` |
| preserved-positive | Large reports still fire the trigger | 6/10 | `go test -tags acs -run TestC2141_004 ./acs/cycle2141` |
| replay-record | Spec records the replay and the threshold | 6/10 | `go test -tags acs -run TestC2141_006 ./acs/cycle2141` |
