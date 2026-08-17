---
score_cap:
  - criterion: "composeCorrection carries the rejection reason byte-for-byte, exactly once, across multi-line, unicode, and format-verb inputs"
    max_if_missing: 8
    evidence: "go -C go test -count=1 -run '^TestComposeCorrection_CarriesReasonVerbatim$' ./internal/core"
  - criterion: "the verbatim reason stays framed: rejection notice before it, remediation and scope constraint after it"
    max_if_missing: 6
    evidence: "go -C go test -count=1 -run '^TestComposeCorrection_FramingSurroundsTheReason$' ./internal/core"
  - criterion: "composeCorrection's production source is unchanged — the property is green-from-birth and no product edit may claim to establish it"
    max_if_missing: 7
    evidence: "test -z \"$(git diff origin/main -- go/internal/core/retry_backoff.go)\""
---

# Eval: contract-correction verbatim output fidelity

> Locks the verbatim-inclusion property of `composeCorrection`
> (`go/internal/core/retry_backoff.go:12-17`): the deliverable-gate rejection
> reason must reach the re-dispatched phase byte-for-byte, unreformatted,
> untruncated, un-re-wrapped, and appearing exactly once.
>
> Why verbatim is load-bearing. The reason is `deliverable.summarize()`'s
> rendering, carrying `[code] message` tokens. `contractViolationCodeRE` parses
> those same tokens back out downstream in `contract_escalation.go` to compute
> contract-block identity, and the re-dispatched agent reads the literal
> violation text. A lossy transform breaks both consumers at once — the
> escalation ladder silently and the agent visibly.
>
> This eval is deliberately a REGRESSION LOCK, not a change gate. The property
> already held on the pre-change code. Cycle-1508's audit FAIL (defect M1,
> `inst-L1508b`) was caused by claiming a tautologically-green criterion as work
> "established" by a product change; the third score_cap row encodes the
> correction by requiring `retry_backoff.go` itself to stay untouched. Source
> incident: cycle-1508.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| verbatim-inclusion | Reason survives byte-for-byte and exactly once (multi-line summarize renderings, unicode, `%s`/backslash content, whitespace edges) | 8/10 | `go test -run TestComposeCorrection_CarriesReasonVerbatim ./internal/core` |
| framing-structure | Rejection framing precedes the reason; remediation + "do not change unrelated files" follow it — so a degenerate "return the bare reason" implementation cannot satisfy verbatim-inclusion alone | 6/10 | `go test -run TestComposeCorrection_FramingSurroundsTheReason ./internal/core` |
| no-tautological-claim | `retry_backoff.go` is unmodified vs the cycle base — the deliverable is the lock, not a behaviour change | 7/10 | `git diff origin/main -- go/internal/core/retry_backoff.go` is empty |
