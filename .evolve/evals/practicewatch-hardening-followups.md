---
score_cap:
  - criterion: "sanitizeUntrusted strips Unicode format characters (Cf: bidi overrides, zero-width, BOM) from the filed item summary"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1909_001 ./acs/cycle1909"
  - criterion: "an unreadable inbox directory is a StatusError raised by idFiled before any file attempt, not counted as not-filed"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1909_002 ./acs/cycle1909"
  - criterion: "an HTTP body or command output cut at the 8 MiB limit carries a truncated marker in the row detail"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1909_00[34]' ./acs/cycle1909"
  - criterion: "a body within the limit (exactly 8 MiB, empty, small) carries no truncated marker, and a readable inbox still files"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1909_00[56]' ./acs/cycle1909"
  - criterion: "the practicewatch package stays gofmt-clean, vet-clean and green"
    max_if_missing: 5
    evidence: "cd go && test -z \"$(gofmt -l internal/practicewatch)\" && go vet ./internal/practicewatch/... && go test -count=1 ./internal/practicewatch/..."
---

# Eval: Practice watch hardening follow-ups

> Pins three LOW findings of the practice watch review (M1 delta review, 2026-10-12), fixed in cycle 1909.
> Format characters survived sanitization, idFiled discarded a WalkDir error, and the 8 MiB cap cut bodies silently.
> Each predicate drives the production entry point `practicewatch.Watch` with a real HTTP server or command runner.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cf-strip | Cf runes removed, visible text kept | 7/10 | `go test -tags acs -run TestC1909_001 ./acs/cycle1909` |
| walk-error | unreadable inbox is an error before filing | 7/10 | `go test -tags acs -run TestC1909_002 ./acs/cycle1909` |
| truncation-marker | HTTP and command bodies over 8 MiB say truncated | 7/10 | `go test -tags acs -run 'TestC1909_00[34]' ./acs/cycle1909` |
| no-false-marker | within-limit bodies and readable inboxes unchanged | 5/10 | `go test -tags acs -run 'TestC1909_00[56]' ./acs/cycle1909` |
| hygiene | gofmt, vet, package tests | 5/10 | `gofmt -l`, `go vet`, `go test` |
