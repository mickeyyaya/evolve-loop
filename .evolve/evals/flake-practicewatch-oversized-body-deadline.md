---
score_cap:
  - criterion: "The truncation Watch tests set a Timeout of at least 30 s"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2103_001 ./acs/cycle2103"
  - criterion: "A test proves a slow source ends in StatusError with the deadline text"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2103_002 ./acs/cycle2103"
  - criterion: "The truncation Watch test passes under -race -count=50"
    max_if_missing: 6
    evidence: "cd go && go test -race -count=50 -timeout 15m -run '^TestWatch_AnOversizedBodyIsRecordedWithATruncationNote$' ./internal/practicewatch"
---

# Eval: practicewatch oversized-body test deadline

> Pins the fix for the flake in `TestWatch_AnOversizedBodyIsRecordedWithATruncationNote`. The shared `optionsFor` fixture gives a 3 s fetch deadline. An 8 MiB body under `-race` load can exceed it and the watch ends in StatusError instead of StatusRecorded. The fix gives the truncation tests a Timeout of 30 s or more and keeps a test that proves the deadline path. Source incident: cycle 2103 inbox item flake-practicewatch-oversized-body-deadline.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| timeout-floor | Truncation Watch tests have Timeout of 30 s or more | 6/10 | `go test -tags acs -run TestC2103_001 ./acs/cycle2103` |
| deadline-path | A slow source ends in StatusError with the deadline text | 7/10 | `go test -tags acs -run TestC2103_002 ./acs/cycle2103` |
| race-stability | The truncation Watch test passes `-race -count=50` | 6/10 | `go test -race -count=50 -timeout 15m -run '^TestWatch_AnOversizedBodyIsRecordedWithATruncationNote$' ./internal/practicewatch` |
