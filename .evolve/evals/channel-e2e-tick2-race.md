---
score_cap:
  - criterion: "TestChannel_EndToEnd passes when stdout is read before the assistant text lands and stderr after idle_reached"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2052_001 ./acs/cycle2052"
  - criterion: "TestChannel_EndToEnd passes 200 times with -race"
    max_if_missing: 7
    evidence: "cd go && go test -race -count=200 -run '^TestChannel_EndToEnd$' ./internal/bridge/channel/"
  - criterion: "TestChannel_EndToEnd fails when response_complete closes the span before the assistant text"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2052_003 ./acs/cycle2052"
---

# Eval: channel-e2e-tick2-race

## Graders
- [code] `cd go && go test -race -count=200 -run TestChannel_EndToEnd ./internal/bridge/channel/` exits 0.
- [code] `cd go && go vet ./internal/bridge/channel/ && test -z "$(gofmt -l internal/bridge/channel)"` exits 0.
- [code] Negative: in e2e_test.go the idle_reached append happens only after waitForFeed observed the assistant text in the feed; `grep -n 'idle_reached' go/internal/bridge/channel/e2e_test.go` shows it after a waitForFeed call for the assistant text.
- [code] Edge/mutation: moving the idle_reached append before the assistant-text append makes the test fail (exit non-zero), proving the test probes the ordering.
- [code] `git diff --name-only main...HEAD` lists only go/internal/bridge/channel/e2e_test.go.
- [model] Test has no sleeps added and no code comments.


> Source incident: PR #874 CI run 38241358938; cycle 2052 pins the worst interleaving through a normalizer overlay in go/acs/cycle2052.
