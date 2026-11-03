---
score_cap:
  - criterion: "agent run exits with the usage code and starts no seat when --writable names a directory"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestAgentRun_TheBuildSeatRefusesAWritableThatNamesADirectory$' ./internal/cli/agentcmd"
  - criterion: "agent run still accepts a --writable that names a file that does not exist yet, and a file that exists"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestAgentRun_TheBuildSeatAcceptsAWritableThatIsAFileOrNotYetThere$' ./internal/cli/agentcmd"
  - criterion: "the usage text of agent run states that --writable names a file, never a directory"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestAgentRun_TheUsageStatesThatWritableNamesAFileNeverADirectory$' ./internal/cli/agentcmd"
  - criterion: "the agent-run contract document states the directory rule on a line that names --writable"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2131_005_' ./acs/cycle2131"
---

# Eval: agent run refuses a --writable that names a directory

> Pins the rule that a `--writable` path of `evolve agent run` names one file. A directory entry
> lets the fence keep every change under it, which is wider than the exact-file grant that
> `docs/architecture/agent-run.md` §8 states. Source: inbox item `agent-run-refuses-a-directory-writable`, cycle 2131.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| directory-refusal | `--writable go` exits 10 and starts no seat | 3/10 | `go test -run TestAgentRun_TheBuildSeatRefusesAWritableThatNamesADirectory` |
| new-file-preserved | a new or existing file still passes | 5/10 | `go test -run TestAgentRun_TheBuildSeatAcceptsAWritableThatIsAFileOrNotYetThere` |
| usage-states-rule | the usage names the rule in STE | 6/10 | `go test -run TestAgentRun_TheUsageStatesThatWritableNamesAFileNeverADirectory` |
| doc-states-rule | agent-run.md states the rule | 6/10 | `go test -tags acs -run TestC2131_005_ ./acs/cycle2131` |
