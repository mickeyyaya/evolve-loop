---
score_cap:
  - criterion: "A writer seat that creates a file outside its writable list ends with a verdict that is not PASS, and the signal names the path and the next step"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC2357_001|TestC2357_007' ./acs/cycle2357/"
  - criterion: "A new file in a granted directory survives and the seat passes; a sibling directory and a stray beside the grant are still removed"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC2357_004|TestC2357_005' ./acs/cycle2357/"
  - criterion: "An existing file outside the grant is still restored"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC2357_003' ./acs/cycle2357/"
  - criterion: "The directory grant has a production caller: evolve agent run accepts --writable-dir, refuses an unsafe one and states it in the usage"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestAgentRun_TheBuildSeat(KeepsANewFile|AcceptsAWritableDir|RefusesAWritableDir)|TestAgentRun_(AReaderRefusesAWritableDir|TheUsageStatesTheWritableDirGrant)' ./internal/cli/agentcmd/"
---

# Eval: A build seat's new files are erased by the fence, and the seat still reports PASS

> Pins the repair of the cycle-2357 task `builder-fence-erases-new-files-and-the-seat-says-pass`.
> The worktree fence removed a file that a writer seat created outside its writable list.
> It logged only a warning, and the seat verdict stayed PASS. The seat then reported work that the tree no longer held.
> The grant is now explicit: a file by `--writable`, a directory for new files by `--writable-dir`.
> A removed seat-created file must end the `evolve agent run` seat with a verdict that is not PASS (envelope, exit code and launch record), with an error diagnostic that names the path and the `--writable` next step.
> The audit of the first build rejected a version that only added the diagnostic (finding H1): the verdict half of criterion 1 is graded verbatim.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| journey | Created file outside the grant ends the seat with a verdict that is not PASS, a non-zero exit, and a signal with the path and the next step | 3/10 | `go test -tags acs -run 'TestC2357_001|TestC2357_007' ./acs/cycle2357/` |
| grant | New file in a granted directory survives, sibling and stray are removed | 5/10 | `go test -tags acs -run 'TestC2357_004|TestC2357_005' ./acs/cycle2357/` |
| preservation | Existing file outside the grant is restored | 6/10 | `go test -tags acs -run TestC2357_003 ./acs/cycle2357/` |
| caller | `--writable-dir` reaches the fence from the CLI | 5/10 | `go test -run 'TestAgentRun_TheBuildSeatKeepsANewFile...' ./internal/cli/agentcmd/` |
