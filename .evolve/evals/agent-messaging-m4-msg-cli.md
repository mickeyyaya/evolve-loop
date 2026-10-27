---
score_cap:
  - criterion: "the exit codes of evolve msg match the table of ADR-0129 section 11"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestMsgCLI_ExitCodesMatchTheTable$' ./cmd/evolve"
  - criterion: "send reads the body from stdin and exits 2 when no receipt arrives by the deadline"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestMsgSend_ReadsTheBodyFromStdin|TestMsgSend_NoReceiptByTheDeadlineExitsTwo)$' ./cmd/evolve"
  - criterion: "wait wakes on the mailbox through the kernel waker and the source holds no poll timer"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestMsgWait_WakesOnTheMailboxWithoutAPoll|TestMsgWait_SourceHasNoPollTimer)$' ./cmd/evolve"
  - criterion: "reply to an id outside the mailbox exits 4 and writes no draft"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestMsgReply_OutOfScopeExitsFour|TestMsgReply_AnswersTheSenderWithTheCorrelationID)$' ./cmd/evolve"
  - criterion: "the msg verb is registered in the command table"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestMsgCLI_IsRegisteredInTheCommandTable$' ./cmd/evolve"
---

# Eval: Agent messaging M4, the evolve msg CLI

> Pins the `evolve msg send|wait|reply` surface of ADR-0129 section 11. The tests drive the
> real M3 outbox watcher over a fake broker sender, so a draft that the watcher rejects
> fails here. Source: inbox item `agent-messaging-m4-msg-cli`, cycle 2058.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exit-table | each exit code 0 to 5 and 10 | 4/10 | `go test -run TestMsgCLI_ExitCodesMatchTheTable ./cmd/evolve` |
| send | stdin body, deadline exit 2 | 5/10 | `go test -run TestMsgSend ./cmd/evolve` |
| wait | kernel wake, no poll timer | 6/10 | `go test -run TestMsgWait ./cmd/evolve` |
| reply | scope exit 4, correlation id | 6/10 | `go test -run TestMsgReply ./cmd/evolve` |
| registry | verb reachable from the command table | 5/10 | `go test -run TestMsgCLI_IsRegistered ./cmd/evolve` |
