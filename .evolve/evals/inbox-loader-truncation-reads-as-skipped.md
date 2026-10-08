---
score_cap:
  - criterion: "A loaded-but-truncated item (overlong title, acceptance or files entry) prints as truncated, never as skipped, in both evolve inbox batches and evolve inbox quarantine list"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1837_001_' ./acs/cycle1837"
  - criterion: "A loaded item whose control characters were replaced never prints as skipped"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1837_002_' ./acs/cycle1837"
  - criterion: "An unparsable or wrong-shape record still prints as skipped (never truncated), the verb still exits 0, and every loaded item, truncated ones included, is still counted"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1837_003_' ./acs/cycle1837"
---

# Eval: inbox loader warnings say truncated for loaded items and skipped only for unreadable ones

> Pins inbox item inbox-loader-truncation-reads-as-skipped. `loadPendingInbox`
> (cmd_inbox.go, shared by batches/list/rank/show) and `inbox quarantine list`
> (cmd_inbox_quarantine.go) prefixed every `inboxbatch.LoadDir` warning with
> `WARN skipped`, but LoadFile's sanitization notice is for an item that loaded
> with fields cut to bound (title/files > 160 bytes, acceptance > 600). On
> 2026-09-30, 44 of 241 items printed as skipped while all 241 were batched, so
> the operator read a dropped queue. Source incident: the 2026-09-30 console
> filing; RED contract authored in cycle 1837. The record's third criterion
> (the checked-in inbox prints no truncation warning) needs the over-length
> items shortened at a boundary and is deferred (see the cycle 1837
> test-report.md AC-Materialization table).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| truncated-not-skipped | Overlong title/acceptance/files item prints as truncated, never skipped, in both verbs | 4/10 | `go test -tags acs -run '^TestC1837_001_' ./acs/cycle1837` |
| sanitized-not-skipped | Control-character item never prints as skipped | 6/10 | `go test -tags acs -run '^TestC1837_002_' ./acs/cycle1837` |
| unparsable-still-skipped | Unparsable/wrong-shape record prints as skipped, loaded count unchanged | 5/10 | `go test -tags acs -run '^TestC1837_003_' ./acs/cycle1837` |
