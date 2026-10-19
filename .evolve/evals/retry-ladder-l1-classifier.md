---
score_cap:
  - criterion: "The fixture cycles classify by cause: 1946, 1950 and 1953 as spec; 1965 and 1966 as capability"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestClassify_FixtureCycles ./internal/retryladder/"
  - criterion: "Exits 2, 3, 10, 86 and 99 classify as halt; exit 85 without a bench is infra; exit 85 with a bench is quota"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestClassify_ExitCodes ./internal/retryladder/"
  - criterion: "A failure that matches no signal is unknown"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestClassify_SpecAndCapabilitySignals/no_signal ./internal/retryladder/"
  - criterion: "The retryladder package does not import go/internal/core"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run TestC1973_003 ./acs/cycle1973/"
  - criterion: "The fingerprint is the violation-code set, else the normalized text"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestFingerprint ./internal/retryladder/"
---

# Eval: Retry ladder L1, the deterministic failure classifier

> Pins `retryladder.Classify` and `retryladder.Fingerprint` of ADR-0142 component L1 (spec `docs/architecture/retry-ladder.md` §3). The classifier reads host facts only and keeps the `core` package out of its imports. Source incident: the aborted builds of cycles 1946, 1950, 1953, 1965 and 1966 (research F2.11), where a stronger model could not fix the three spec failures.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fixture-cycles | five aborted cycles classify to their F2.11 class | 6/10 | `go test -run TestClassify_FixtureCycles` |
| exit-codes | halt, infra and quota exits | 7/10 | `go test -run TestClassify_ExitCodes` |
| unknown-residue | no signal gives unknown | 6/10 | `go test -run TestClassify_SpecAndCapabilitySignals/no_signal` |
| no-core-import | `go list -deps` has no `internal/core` | 8/10 | `go test -tags acs -run TestC1973_003 ./acs/cycle1973/` |
| fingerprint | code-set identity, else normalized text | 5/10 | `go test -run TestFingerprint` |
