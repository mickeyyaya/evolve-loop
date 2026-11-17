---
score_cap:
  - criterion: "a one-field edit of an item filed by the inbox filer changes exactly the weight line"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2478_001_' ./acs/cycle2478"
  - criterion: "a criterion or text value containing an id token survives an edit byte-for-byte, with no HTML escaping, on set, add and list-set paths"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2478_002_|TestC2478_004_' ./acs/cycle2478"
  - criterion: "the runtime reference no longer says the item writer rewrites compact JSON with sorted keys"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2478_003_' ./acs/cycle2478"
---

# Eval: The inbox item writer keeps the filed format

> Pins the filed-format contract of the inbox item writer. The rewrite of an item must change only the edited line and must not HTML-escape text. Cycle 2478 found that the writer keeps the indent but `jsonString` and `jsonStrings` in the lifecycle package escape angle brackets and ampersands in an edited value. The runtime reference still described a compact rewrite.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| one-line diff | a weight edit changes one line | 7/10 | `go test -tags acs -run TestC2478_001_ ./acs/cycle2478` |
| byte-for-byte text | an id token in a value is not escaped | 8/10 | `go test -tags acs -run 'TestC2478_002_|TestC2478_004_' ./acs/cycle2478` |
| doc truth | the caveat is gone | 5/10 | `go test -tags acs -run TestC2478_003_ ./acs/cycle2478` |
