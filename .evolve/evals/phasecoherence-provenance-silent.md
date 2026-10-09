---
score_cap:
  - criterion: "A ledger record over 64 KiB is scanned and the entries after it are cross-checked, never a silent partial scan"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1851_001' ./acs/cycle1851"
  - criterion: "A malformed ledger line is reported as exactly one WARN violation"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1851_00[23]' ./acs/cycle1851"
  - criterion: "An unreadable ledger yields an error or a violation"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1851_004' ./acs/cycle1851"
  - criterion: "Ledger role matching is case-insensitive (Build matches build) and different roles still do not match"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1851_00[56]' ./acs/cycle1851"
  - criterion: "The phasecoherence package suite stays green"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/phasecoherence/"
---

# Eval: phasecoherence CheckProvenance reports ledger problems and matches roles case-insensitively

> Pins the fix for inbox item phasecoherence-provenance-silent (batch 14, 2026-09-26; cycle 1851).
> CheckProvenance stopped at the first ledger line over bufio's 64 KiB default, skipped malformed
> JSON lines without a count, and canonicalRole mapped aliases only in lowercase, so a ledger role
> "Build" never matched phase "build".

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| oversize-line | >64 KiB record scanned | 7/10 | `go test -tags acs -run TestC1851_001` |
| malformed-line | 1 WARN violation; clean ledger none | 6/10 | `go test -tags acs -run 'TestC1851_00[23]'` |
| unreadable | error or violation | 6/10 | `go test -tags acs -run TestC1851_004` |
| case-insensitive | Build==build, Scout!=build | 7/10 | `go test -tags acs -run 'TestC1851_00[56]'` |
| no-regression | package suite green | 8/10 | `go test -count=1 ./internal/phasecoherence/` |
