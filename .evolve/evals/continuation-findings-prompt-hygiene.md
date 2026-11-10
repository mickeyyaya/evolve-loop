---
score_cap:
  - criterion: "Control bytes in the failure phase and reasons never reach the continuation findings block"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestReadContinuationFindings_StripsControlBytesFromPhaseAndReasons$' ./internal/core/"
  - criterion: "Control bytes in the raw-body fallback never reach the continuation findings block"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestReadContinuationFindings_StripsControlBytesFromTheRawBodyFallback$' ./internal/core/"
  - criterion: "A reason with embedded newlines cannot forge extra lines in the findings block"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestReadContinuationFindings_AReasonCannotForgeAnotherLine$' ./internal/core/"
  - criterion: "The findings truncation never splits a multibyte rune"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestReadContinuationFindings_NeverSplitsARuneAtTheBudget|TestTruncateFindings_NeverSplitsARuneAndReportsTheDroppedBytes)$' ./internal/core/"
  - criterion: "A body with no readable text after sanitizing serves no findings"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestReadContinuationFindings_ABodyOfOnlyControlBytesServesNothing$' ./internal/core/"
  - criterion: "The sanitized findings reach the build request and the console adoption brief through their production callers"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestRunCycle_AContinuationBuildContextCarriesNoControlBytesFromTheFailureReasons|TestConsoleFindings_CarryNoControlBytesFromTheFailureReasons)$' ./internal/core/"
---

# Eval: Sanitize the prior-attempt findings block of a continuation

> Pins the control-character hygiene of `readContinuationFindings` in `go/internal/core/continuation_stamp.go`. The structure half (reasons rendered as lines, line-boundary truncation) landed in #370. The open half is the sanitizer: ADR-0076 slice C architect finding 4 asked for the same control-character hygiene that `inboxbatch` applies. Source: inbox item `continuation-findings-prompt-hygiene`, cycle 2277.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| reasons-sanitized | Phase and reasons carry no control byte | 6/10 | `TestReadContinuationFindings_StripsControlBytesFromPhaseAndReasons` |
| raw-body-sanitized | Raw-body fallback carries no control byte | 6/10 | `TestReadContinuationFindings_StripsControlBytesFromTheRawBodyFallback` |
| no-line-forgery | A reason stays on one line | 7/10 | `TestReadContinuationFindings_AReasonCannotForgeAnotherLine` |
| rune-safe-cut | The budget cut keeps valid UTF-8 | 6/10 | `TestTruncateFindings_NeverSplitsARuneAndReportsTheDroppedBytes` |
| empty-after-strip | Control-only body serves nothing | 5/10 | `TestReadContinuationFindings_ABodyOfOnlyControlBytesServesNothing` |
| production-callers | Both callers serve sanitized text | 7/10 | `TestRunCycle_AContinuationBuildContextCarriesNoControlBytesFromTheFailureReasons` |
