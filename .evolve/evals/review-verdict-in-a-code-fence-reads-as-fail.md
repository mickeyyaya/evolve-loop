---
score_cap:
  - criterion: "A review report whose only verdict line is in a code fence reads as the verdict that it states, through evolve agent run"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestAgentRun_AVerdictLineInACodeFenceReadsAsTheVerdictThatItStates|TestReportVerdict_AFencedVerdictStatesTheVerdictThatItReads)$' ./internal/cli/agentcmd"
  - criterion: "A visible CRITICAL or HIGH finding still fails the run when the verdict line sits in a fence"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestAgentRun_AVisibleHighWithAFencedPassVerdictStaysFail$' ./internal/cli/agentcmd"
  - criterion: "A report with no verdict at all still fails and emits the parse defect"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestAgentRun_AReportWithNoVerdictInOrOutOfAFenceStillFails|TestAgentRun_NoStatedVerdictWithNoBlockingFindingEmitsAParseDefect|TestReportVerdict_TheParseDefectNeedsNoStatedVerdictAndNoBlockingFinding)$' ./internal/cli/agentcmd"
  - criterion: "A fully fenced report with a blocking finding and a fenced PASS or WARN stays FAIL (audit H1)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestAgentRun_AFullyFencedReportWithABlockingFindingStaysFail|TestReportVerdict_AFencedBlockingFindingBlocksTheVerdictThatTheFenceStates|TestReportVerdict_AVisibleVerdictKeepsAFencedExampleFindingUncounted)$' ./internal/cli/agentcmd"
---

# Eval: A verdict in a code fence reads as the verdict it states

> Pins the fence-aware verdict read of `evolve agent run` (`go/internal/cli/agentcmd/verdict.go`). A review seat
> wrote `Verdict: PASS` inside a code fence; the sentinel reader and `reportdoc.Verdict` both skip fences, so the
> run read no verdict, set FAIL and emitted `REVIEW_AGENT_RUN_VERDICT_PARSE_DEFECT`. Source incident: cycle 2473,
> run `agent-code-reviewer-20261105T233200Z-ea1f`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fenced-verdict | A fenced PASS or FAIL is read as stated | 4/10 | `TestAgentRun_AVerdictLineInACodeFenceReadsAsTheVerdictThatItStates` |
| blocking-kept | A visible HIGH still fails | 6/10 | `TestAgentRun_AVisibleHighWithAFencedPassVerdictStaysFail` |
| no-verdict | No verdict still fails | 6/10 | `TestAgentRun_AReportWithNoVerdictInOrOutOfAFenceStillFails` |
| fenced-blocking | A fully fenced HIGH with a fenced PASS stays FAIL (audit H1) | 5/10 | `TestAgentRun_AFullyFencedReportWithABlockingFindingStaysFail` |
