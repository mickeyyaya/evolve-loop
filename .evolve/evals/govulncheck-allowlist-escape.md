---
score_cap:
  - criterion: "vulnfilter fails closed on an unlisted finding and passes an allowlisted one"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1881_00[2-4]' ./acs/cycle1881"
  - criterion: "expired or reasonless allowlist entries fail the run"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1881_00[5-7]' ./acs/cycle1881"
  - criterion: "make vulncheck pipes govulncheck -json into vulnfilter with the tracked allowlist"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1881_01[3-6]' ./acs/cycle1881"
  - criterion: "make vulncheck fails when govulncheck itself fails, even when the filter sees a clean or empty stream"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run TestC1881_019 ./acs/cycle1881"
  - criterion: "vulnfilter sources carry no comments"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1881_017 ./acs/cycle1881"
---

# Eval: govulncheck allowlist escape

> Pins the emergency escape for `make vulncheck`: findings from `govulncheck -json` are filtered by `go/cmd/vulnfilter` against the tracked `go/vulncheck-allowlist.json` (id, expires, reason). Unlisted findings, expired entries and reasonless entries fail the gate. Source: inbox item govulncheck-allowlist-escape, cycle 1881.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| filter-core | unlisted fails, allowlisted passes | 4/10 | `go test -tags acs -run TestC1881_00[2-4]` |
| waiver-hygiene | expired and reasonless entries fail | 5/10 | `go test -tags acs -run TestC1881_00[5-7]` |
| wiring | Makefile pipes -json into vulnfilter | 5/10 | `go test -tags acs -run TestC1881_01[3-6]` |
| scanner-failure propagation | a failing govulncheck fails the gate (audit round 1, H1) | 3/10 | `go test -tags acs -run TestC1881_019` |
| convention | zero comments | 6/10 | `go test -tags acs -run TestC1881_017` |
