---
score_cap:
  - criterion: "A claude transcript of 3 assistant messages gives 3 ordered turns with counts and tool names"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestTurns_ClaudeTranscriptGivesOneTurnPerMessageInTimeOrder|TestTurns_ClaudeTurnSumEqualsTheRowUsage' ./internal/tokenusage/"
  - criterion: "The cycle-1967 agy conversation gives 111 turns whose sum equals the row of the call"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestTurns_Agy14ConversationGivesOneTurnPerGenerationSummingToTheRow|TestEngineLaunch_Agy14TurnsFillTheSidecarAndSumToTheRow' ./internal/tokenusage/ ./internal/bridge/"
  - criterion: "A call above the line cap writes the cap and a truncated count"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestAppendTurns_CallAboveTheCapWritesTheCapAndATruncatedCount|TestEngineLaunch_CallAboveTheLineCapWritesTheCapAndATruncatedCount' ./internal/llmcalls/ ./internal/bridge/"
  - criterion: "A turn sum that differs from the row fires a WARN and leaves the row unchanged"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestEngineLaunch_TurnSumMismatchWarnsAndLeavesTheRowAlone ./internal/bridge/"
  - criterion: "A turn write failure warns and never fails the launch"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestEngineLaunch_TurnWriteFailureWarnsAndNeverFailsTheLaunch ./internal/bridge/"
  - criterion: "Parallel tool calls of one claude message keep every tool name"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestTurns_ParallelToolCallsOfOneMessageKeepEveryToolNameInOrder|TestTurns_ToolNamesSpreadOverTextAndToolLinesKeepOnlyToolUse|TestTurns_RepeatedToolUseLineOfOneMessageIsCountedOnce' ./internal/tokenusage/"
  - criterion: "The tokenusage package note says the tool names of a turn come from every in-window line of its message id"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2185_006 ./acs/cycle2185"
---

# Eval: Turn sidecar llm-turns.ndjson

> Pins the turn sidecar of plan docs/plans/token-telemetry-2026-10.md component T3 (cycle 2175). Each assistant message of a claude transcript and each generation of an agy conversation becomes one line keyed by the call_id of the llm-calls row. The row stays one line for each call. The sidecar is telemetry: a failure of it warns and never changes the launch or the row.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| claude-turns | 3 messages give 3 ordered turns with tool names | 6/10 | `go test -run TestTurns_Claude... ./internal/tokenusage/` |
| agy-turns | 111 generations give 111 lines summing to the row | 7/10 | `go test -run TestTurns_Agy14... ./internal/tokenusage/ ./internal/bridge/` |
| line-cap | above the cap: the cap plus a truncated count | 6/10 | `go test -run ...CallAboveTheCap... ./internal/llmcalls/ ./internal/bridge/` |
| sum-check | mismatch warns, the row is unchanged | 7/10 | `go test -run TestEngineLaunch_TurnSumMismatch... ./internal/bridge/` |
| parallel-tools | every tool_use line of a message id keeps its name (audit H1, cycle 2175) | 6/10 | `go test -run TestTurns_ParallelToolCalls... ./internal/tokenusage/` |
| fail-open | write failure warns, the launch passes | 8/10 | `go test -run TestEngineLaunch_TurnWriteFailure... ./internal/bridge/` |
| package-note | note records the all-lines tool-name rule (audit M1, cycle 2175) | 5/10 | `go test -tags acs -run TestC2185_006 ./acs/cycle2185` |
