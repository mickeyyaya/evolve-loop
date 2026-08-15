---
score_cap:
  - criterion: "A `git add` failure whose captured git_stderr carries one of git's deterministic fatals (rc=128 Invalid path / outside repository / pathspec-did-not-match, rc=1 gitignore advice-refusal) is classified non-transient on the FIRST failure"
    max_if_missing: 7
    evidence: "cd go && go test ./internal/phases/ship -run 'TestStageFailureClassification' -count=1"
  - criterion: "Index-lock contention, unrecognised stderr, empty stderr, and Go-composed error text all remain ShipClassTransient — the classifier reads captured git_stderr only, never the message Go wraps around it"
    max_if_missing: 8
    evidence: "cd go && go test ./internal/phases/ship -run 'TestStageFailureClassification/(rc128_index_lock_contention_stays_transient|unrecognised_stderr_degrades_to_transient|empty_stderr_degrades_to_transient|go_error_text_alone_does_not_classify)' -count=1"
  - criterion: "The captured git stderr still travels in ShipError.Debug[git_stderr] (the classifier's own input, and what the failure digest / retro / escalation report read)"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/phases/ship -run 'TestStageFailureClassification_PreservesCapturedStderr' -count=1"
  - criterion: "The cycle-1440 two-strikes refusal router still applies to failures the stderr classifier cannot place — classification sits in FRONT of it, not instead of it"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/phases/ship -run 'TestStageRefusal|TestStageFailureClassification_TwoStrikesStillApplies' -count=1"
---

# Eval: Classify deterministic Git stage failures before retry

> Pins the recovery-class contract of `stageExplicitPaths`
> (`go/internal/phases/ship/gitops.go`). Before this task, EVERY `git add`
> failure was born `core.ShipClassTransient`, so the recovery ladder
> re-dispatched byte-identical adds for failures git had already declared
> unwinnable. Two live instances, same day: cycle-1098 fed git an absolute
> pathspec (`fatal: Invalid path '/go/bin/evolve'`, rc=128) and cycle-1101 fed it
> a gitignored path (`The following paths are ignored by one of your .gitignore
> files`, rc=1) — each retried twice as "transient" before aborting, and each
> failure digest recorded a class that was nothing of the sort. Both root causes
> were fixed console-first; this eval guards the CLASSIFICATION seam, which is
> what stops the next unseen deterministic shape from burning the same budget.
>
> The second cap is the load-bearing one. The cheap wrong fix — reclassify every
> add failure as non-transient — satisfies the first criterion while deleting the
> retry that index-lock contention depends on (fleet lanes contend on
> `.git/index` constantly, and that retry WINS). The same cap pins the trust
> boundary the source inbox record states explicitly: classify the CAPTURED
> `git_stderr`, never the message the Go code composes, so an error-wrapper edit
> can never silently move a failure between recovery routes.
>
> Source incidents: cycle-1098 and cycle-1101 (2026-07-27), inbox
> `gitstage-deterministic-classification`; materialised cycle-1473.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| deterministic-positives | rc=128 fatals + rc=1 advice-refusal classify non-transient on first failure | 7/10 | `go test ./internal/phases/ship -run TestStageFailureClassification` |
| transient-negatives | index-lock / unknown / empty stderr / Go-error-text stay transient | 8/10 | `go test -run 'TestStageFailureClassification/(rc128_index_lock…\|…\|go_error_text_alone_does_not_classify)'` |
| evidence-preservation | `Debug[git_stderr]` still carries the captured stderr | 6/10 | `go test -run TestStageFailureClassification_PreservesCapturedStderr` |
| router-composition | cycle-1440 two-strikes memo survives in front-to-back order | 6/10 | `go test -run 'TestStageRefusal\|…_TwoStrikesStillApplies'` |
