---
score_cap:
  - criterion: "An over-long transcript line yields an error or a Warn instead of silent truncation"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestTranscriptScan_OverlongLine ./internal/tokenusage"
  - criterion: "No tokenusage test compares a value with itself and no hand-rolled itoa or jsonQuote remains"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1847_003|TestC1847_004' ./acs/cycle1847"
  - criterion: "FillWarn keeps its contract for negative and unmeasured readings"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run TestC1847_006 ./acs/cycle1847"
---

# Eval: tokenusage silent reads and vacuous test

> Pins the cycle 1847 fix: `readLines` must surface `bufio.ErrTooLong` rather than under-count tokens, and the tokenusage tests must not contain tautological comparisons or broken hand-rolled helpers.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| overlong-line | error or Warn on a line over 8 MiB | 5/10 | `go test -run TestTranscriptScan_OverlongLine` |
| test-hygiene | no self-comparison, no itoa/jsonQuote | 6/10 | `go test -tags acs -run 'TestC1847_003|TestC1847_004'` |
| fillwarn-preserved | negative readings stay silent | 6/10 | `go test -tags acs -run TestC1847_006` |
