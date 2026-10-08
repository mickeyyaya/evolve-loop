---
score_cap:
  - criterion: "Exactly one non-test string literal spells the inbox filename-stamp layout, declared in internal/inboxbatch, and ciwatch writes its escalation file name through it"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1837_004_' ./acs/cycle1837"
  - criterion: "The ciwatch escalation file name is unchanged and FiledAt's file-name fallback reads its stamp back to the escalation time"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1837_005_' ./acs/cycle1837"
---

# Eval: one filename-stamp layout, written by ciwatch and read by inboxbatch.FiledAt

> Pins inbox item inbox-filename-stamp-single-source (F40 architecture review
> m5 / re-review MINOR 4). `inboxbatch.FilenameStampLayout`
> ("2006-01-02T15-04-05Z") parses the stamp that `ciwatch.fileEscalation`
> wrote with its own bare literal, so a respelling on either side would
> silently turn FiledAt's file-name fallback into zero. The predicate scans
> every non-test, non-vendor Go file's string literals (comments excluded) for
> the dashed time-of-day layout and requires ciwatch to reference
> inboxbatch.FilenameStampLayout or an inboxbatch function built on it. Source
> incident: the F40 review; RED contract authored in cycle 1837.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| single-declaration-pin | One layout literal, in inboxbatch; ciwatch writes through it | 4/10 | `go test -tags acs -run '^TestC1837_004_' ./acs/cycle1837` |
| round-trip-preserved | Escalation name unchanged; FiledAt fallback reads it back | 5/10 | `go test -tags acs -run '^TestC1837_005_' ./acs/cycle1837` |
