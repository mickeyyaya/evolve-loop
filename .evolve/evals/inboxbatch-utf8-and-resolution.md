---
score_cap:
  - criterion: "cleanBounded never returns invalid UTF-8: a multi-byte rune at the byte limit is dropped whole, values at the limit are kept whole"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1809_00[123]_' ./acs/cycle1809"
  - criterion: "a duplicate id resolves to the same earliest-filed item in connects, unified-commitment validation and plan-time routing"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1809_00[4567]_' ./acs/cycle1809"
  - criterion: "a malformed console-routed item's plan-time gate reason carries a coded INBOX_LOAD_WARNING naming its record, without changing any routing decision"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1809_0(08|09|10)_' ./acs/cycle1809"
  - criterion: "fileAreaRule and IsOperatorState read the same tokenized, locator-stripped files[] entries that routing declares"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1809_01[123]_' ./acs/cycle1809"
---

# Eval: inboxbatch UTF-8 truncation and single id resolution

> Pins the four inboxbatch defects of inbox item inboxbatch-utf8-and-resolution: UTF-8-safe truncation, one first-wins id resolution in filing order across connects, validation and routing, rules that read routing's own files[] tokens, and a coded plan-time WARN for a malformed console-routed record. Source incidents: cycles 1804 and 1808 failed on this item because their WARN predicates required an edit to the protected go/internal/loopwave/dispatch.go (inst-L1804b, inst-L1808a). Cycle 1809 asserts the code at inboxbatch.RoutedResolver, whose reason the unchanged gate already prints.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| utf8-cut | no split rune; at-limit values kept whole | 6/10 | `go test -tags acs -run 'TestC1809_00[123]_' ./acs/cycle1809` |
| dup-id | earliest-filed holder wins in connects, Validate and RoutedResolver | 6/10 | `go test -tags acs -run 'TestC1809_00[4567]_' ./acs/cycle1809` |
| coded-warn | INBOX_LOAD_WARNING in the gate reason; fail-open kept | 7/10 | `go test -tags acs -run 'TestC1809_0(08|09|10)_' ./acs/cycle1809` |
| shared-tokens | file-area and operator-state agree with routing's DeclaredPaths | 6/10 | `go test -tags acs -run 'TestC1809_01[123]_' ./acs/cycle1809` |
