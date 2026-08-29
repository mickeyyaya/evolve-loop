---
score_cap:
  - criterion: "A consecutive-failures pipeline-blocker escalation preserves the halt's own evidence and never claims audit/ACS artifacts are green or that a verdict was forged"
    max_if_missing: 8
    evidence: "cd go && go test -run TestWritePipelineEscalation_ConsecutiveFailuresPreservesEvidence ./cmd/evolve"
  - criterion: "A verdict-incoherence escalation still directs diagnosis through recorded verdict vs audit-report.md + acs-verdict.json artifact comparison"
    max_if_missing: 6
    evidence: "cd go && go test -run TestWritePipelineEscalation_VerdictIncoherenceKeepsArtifactGuidance ./cmd/evolve"
  - criterion: "An unrecognized system-failure category retains its evidence and never invents the verdict-incoherence root-cause narrative"
    max_if_missing: 7
    evidence: "cd go && go test -run TestWritePipelineEscalation_UnknownCategoryRetainsEvidenceOnly ./cmd/evolve"
---

# Eval: Category-aware pipeline-escalation evidence rendering

> Pins the fix for the cycle-1577 false-diagnosis defect: `writePipelineEscalation`
> (`go/cmd/evolve/cmd_loop_escalation.go`) previously rendered the same
> verdict-incoherence-specific prose ("both artifacts green", "the verdict was
> forged") for every `pipeline-blocker` `SystemFailureSignal`, regardless of
> which rule actually produced it. Cycle 1577's halt came from the
> consecutive-failures rule (`cmd_loop_blockerbreaker.go`), whose audit
> artifact genuinely records `FAIL` — so the inherited template invented a
> contradiction that never happened and queued a P0 repair against the wrong
> root cause (ADR-0072 requires evidence-based classification). The fix makes
> the escalation/inbox renderer category-aware: only the real
> `verdict-incoherence` category gets the artifact-comparison / forged-verdict
> narrative; every other category (including future, currently-unknown ones)
> surfaces its own `SystemFailureSignal.Evidence` verbatim and invents nothing.
> Source incident: cycle 1577 `pipeline-defect-pipeline-blocker-cycle1577`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| evidence-preservation | consecutive-failures pipeline-blocker keeps its own evidence, no invented green/forged claims | 8/10 | `go test -run TestWritePipelineEscalation_ConsecutiveFailuresPreservesEvidence ./cmd/evolve` |
| artifact-guidance-retained | verdict-incoherence keeps its artifact-comparison next_action | 6/10 | `go test -run TestWritePipelineEscalation_VerdictIncoherenceKeepsArtifactGuidance ./cmd/evolve` |
| unknown-category-safe-default | an unrecognized category still preserves evidence and invents nothing | 7/10 | `go test -run TestWritePipelineEscalation_UnknownCategoryRetainsEvidenceOnly ./cmd/evolve` |
