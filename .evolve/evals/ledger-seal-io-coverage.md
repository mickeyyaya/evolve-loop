---
score_cap:
  - criterion: "internal/adapters/ledger statement coverage is >= 85.0%"
    max_if_missing: 6
    evidence: "cd go && go test -coverprofile=/tmp/ev_ledger_c318.cover ./internal/adapters/ledger/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_ledger_c318.cover | awk '/^total:/{gsub(/%/,\"\",$NF); found=1; ok=($NF+0>=85.0)} END{exit !(found && ok)}'"
  - criterion: "linesEqual coverage is 100% — the unequal-elements / length-mismatch FALSE branch is exercised by a negative test"
    max_if_missing: 7
    evidence: "cd go && go test -coverprofile=/tmp/ev_ledger_c318.cover ./internal/adapters/ledger/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_ledger_c318.cover | awk '$2==\"linesEqual\"{gsub(/%/,\"\",$3); found=1; ok=($3+0>=100)} END{exit !(found && ok)}'"
  - criterion: "the TestSeal* contract suite is green (chain-preservation, resume-after-crash, tamper-detection)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestSeal ./internal/adapters/ledger/... 2>&1 | grep -q '^ok'"
---

# Eval: ledger-seal-io-coverage

> Pins the test depth of the seal I/O helpers in
> `go/internal/adapters/ledger/seal.go` — the crash-recovery paths that protect
> the L3.3 chain-preservation guarantee (gunzip(segments) + live tail is
> byte-identical to the pre-seal ledger). Four helpers were under-covered:
> `writeSegment` (50.0%), `rewriteLive` (52.2%), `readSegment` (71.4%), and
> `linesEqual` (66.7%); the package totaled 82.4%. Low coverage on the
> segment-write / live-rewrite / segment-read / equality paths means a
> regression in the ledger-integrity machinery could ship undetected. Source
> incident: cycle 318 scout coverage scan (`go tool cover -func` on
> adapters/ledger). The fix adds targeted table-driven tests; the `linesEqual`
> 100% cap specifically forces a NEGATIVE test that drives the unequal-case
> FALSE branch (a happy-path-only suite could reach the aggregate floor without
> ever asserting rejection — adversarial-testing SKILL §6).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| coverage-floor | ledger package coverage >= 85.0% | 6/10 | `go tool cover -func ... awk total>=85` |
| negative-branch | linesEqual = 100% (FALSE branch covered) | 7/10 | `go tool cover -func ... awk linesEqual>=100` |
| no-regression | TestSeal* contract suite green | 7/10 | `go test -run TestSeal ... \| grep -q '^ok'` |
