---
score_cap:
  - criterion: "A real premise-challenge verdict sentinel is PARSED into the FAIL the composed-path test routes — no hand-built PhaseResponse substitutes for it"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestJudgmentLessonFullPath_PremiseChallengeSentinelFAILTeachesWithoutHalting$/^real_sentinel_parse_produces_the_FAIL$' ./internal/core"
  - criterion: "The persisted carryover carries the exact objection and is visible through the next-cycle planner context (crossing the storage boundary, not read from memory)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestJudgmentLessonFullPath_PremiseChallengeSentinelFAILTeachesWithoutHalting$/^objection_reaches_next_cycle_planner_context$' ./internal/core"
  - criterion: "Teaching imports no halt vector: state.FailedAt is unchanged and the returned loopAction is non-aborting with a nil error"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestJudgmentLessonFullPath_PremiseChallengeSentinelFAILTeachesWithoutHalting$/^failed_at_unchanged_and_continuation_non_halting$' ./internal/core"
  - criterion: "Fail-open negatives leave NO lesson: a stated PASS, a malformed sentinel, and an absent sentinel each file zero carryover todos"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestJudgmentLessonFullPath_NoLessonWithoutAWellFormedSentinelFAIL$' ./internal/core"
---

# Eval: premise-challenge FAIL reaches failure learning on the REAL composed path

> #479 made a judgment phase's FAIL verdict leave a carryover lesson with no halt
> vector, and #481 made a well-formed verdict sentinel able to produce that FAIL
> instead of being structurally forced to PASS. Both shipped unit-proven in
> ISOLATION: `judgment_lesson_test.go` injects a hand-built `PhaseResponse`, and
> the specrunner sentinel tests stop at classification. Nothing drove one real
> sentinel through parse → verdict → `recordAndBranch` → persisted state →
> next-cycle planner context, so the seam between the two layers could be cut
> while every existing test stayed green — and the symptom would be exactly the
> original defect: a correct objection that teaches nobody. Source incident:
> cycle-1528, whose ignored premise-challenge objection survived only because a
> human copied it into an inbox item by hand; the redesign it forced shipped as
> ADR-0090. This eval pins the composed path permanently, so a later cycle that
> rewires classification or the carryover writer cannot silently re-open it.
>
> Scope note, compiler-proven: the composition is pinned at
> `phasecontract.ParseVerdictSentinelFull` — the parser `specrunner`'s
> `applySentinelStage` itself calls — and NOT at `specrunner.EvaluateClassify`,
> because `internal/phases/specrunner` imports `internal/core`, making that
> import an illegal cycle from a `package core` test. Pinning the shared parser
> is the single-source reading: a drift in the sentinel vocabulary breaks this
> test and the live classifier together.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| real-parse-composition | The routed FAIL is produced by the production sentinel parser, not hand-built | 7/10 | `go test -run '.../real_sentinel_parse_produces_the_FAIL'` |
| persistence-reaches-planner | Objection survives the storage boundary into next-cycle planner context | 6/10 | `go test -run '.../objection_reaches_next_cycle_planner_context'` |
| teach-without-halting | `FailedAt` unchanged; continuation non-aborting (no `failureadapter` halt vector) | 4/10 | `go test -run '.../failed_at_unchanged_and_continuation_non_halting'` |
| fail-open-negatives | Stated PASS, malformed, and absent sentinels leave zero lessons | 5/10 | `go test -run '^TestJudgmentLessonFullPath_NoLessonWithoutAWellFormedSentinelFAIL$'` |
