---
score_cap:
  - criterion: "A guard in the core test binary fails when the weighted git starts of the core test files exceed the recorded count"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestCoreTestGitStartsStayInsideTheRecordedCount$' ./internal/core | grep -q -- '--- PASS: TestCoreTestGitStartsStayInsideTheRecordedCount'"
  - criterion: "The git start counter and its verify step have their own passing tests"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/gitstarts"
  - criterion: "The recorded count file exists and is valid JSON with a recorded field"
    max_if_missing: 6
    evidence: "cd go && grep -q '\"recorded\"' internal/core/testdata/gitstarts.json"
---

# Eval: The core tests start fewer git processes

> The core test files start git more than any other package, and the process starts cost kernel time on macOS. Cycle 2577 tried a whole-binary count and a kernel-time ratio, and both failed: the count ran 232 seconds and the ratio was flaky. This eval pins the cheap form instead: a static count of git-start call sites in the `internal/core` test files, checked against a recorded number that may only shrink.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| guard-present | The core guard test runs and passes | 6/10 | `go test -run TestCoreTestGitStartsStayInsideTheRecordedCount ./internal/core` |
| counter-tested | The counter package has its own passing tests | 7/10 | `go test ./internal/gitstarts` |
| baseline-file | The recorded count file exists | 6/10 | `grep recorded internal/core/testdata/gitstarts.json` |
