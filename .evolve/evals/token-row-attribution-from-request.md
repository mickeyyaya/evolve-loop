---
score_cap:
  - criterion: "A row from a request with Cycle 2040, Profile builder and Attempt 2 has cycle 2040, profile builder and purpose retry"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./internal/bridge -run '^(TestRecordModelAttempt_RowCarriesCycleRunProfileAndRetryPurpose|TestEngineLaunch_RowCarriesTheRequestAttribution)$'"
  - criterion: "A request with a correction directive gives purpose contract_correction"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/bridge -run '^TestRecordModelAttempt_PurposeDerivation$'"
  - criterion: "An old row without the new fields decodes unchanged through Read and Import"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/llmcalls -run '^(TestRead_OldRowWithoutAttributionDecodesUnchanged|TestImport_OldRowCopiesByteForByteAndAttributedRowKeepsItsFields)$'"
  - criterion: "At shadow no signal fires; at enforce a cycle row with cycle 0 fires TOKENS_ROW_UNATTRIBUTED once"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/bridge -run '^(TestRecordModelAttempt_ShadowStageFiresNoSignalForACycleRowWithoutCycle|TestRecordModelAttempt_EnforceFiresOnceForACycleRowWithoutCycle|TestRecordModelAttempt_EnforceStaysQuietWhenTheRowIsAttributedOrNotACycleRow)$'"
---

# Eval: token-row-attribution-from-request

> Pins D1 of docs/plans/token-telemetry-2026-10.md: the bridge writes `cycle`, `run_id`, `profile` and a derived `purpose` on each llm-calls row from the current request, and signals `TOKENS_ROW_UNATTRIBUTED` at enforce. Source: cycle 2056, fleet lane for the T1 inbox item.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| attribution-copy | cycle, run id, profile and retry purpose on the row | 4/10 | `go test ./internal/bridge -run TestRecordModelAttempt_RowCarries...` |
| purpose-derivation | correction directive gives contract_correction | 5/10 | `go test ./internal/bridge -run TestRecordModelAttempt_PurposeDerivation` |
| old-row-decode | rows without the new fields decode unchanged | 5/10 | `go test ./internal/llmcalls -run TestRead_OldRow...` |
| stage-gated-signal | shadow quiet, enforce fires once | 6/10 | `go test ./internal/bridge -run TestRecordModelAttempt_Enforce...` |
