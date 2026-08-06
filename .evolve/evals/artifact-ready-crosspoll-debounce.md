---
score_cap:
  - criterion: "the debounce unit tests that back this durable regression lock actually pass"
    max_if_missing: 7
    evidence: "cd go && go test ./internal/bridge/... -run 'TestArtifactDetector_ReadyOnlyAfterCrossPollStability|TestArtifactDetector_NotReadyWhileArtifactStillGrowing|TestArtifactDetector_NotReadyOnSameSizeRewrite|TestArtifactStableTicks_IsAMeaningfulWindow' -v"
  - criterion: "the cross-poll relocation/stability tests (fallback-file growth racing the debounce) also pass"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/bridge/... -run 'TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing|TestArtifactDetector_RelocationHappensOnceFallbackSettles|TestArtifactDetector_RelocatedCompleteFallbackStillCompletes' -v"
---

# Eval: cross-poll artifact-ready debounce must have a durable regression lock

> Source incident: cycle-1198 — a tmux REPL wait loop declared a phase artifact
> (e.g. `build-report.md`) "ready" the instant it appeared on disk, before the
> writer had finished flushing it; the loop moved on and read a truncated
> deliverable. The fix (`artifactDetector` in `go/internal/bridge`) requires the
> artifact's size to stay stable across a debounce window (`ArtifactStableTicks`)
> before declaring it ready, and treats the fallback file's growth/relocation as
> part of the same stability check — so cross-polling between the artifact path
> and its fallback can't race the debounce into a premature "ready".
>
> This debounce has since been carried forward by salvage alone across cycles
> 1233 → 1249 → 1252 → 1254 → 1258 with no permanent artifact binding it: each
> cycle's own build/test run proved the code correct in the moment, but nothing
> capped a FUTURE audit's score if the debounce regressed (e.g. someone drops
> the stability-tick loop while refactoring `RunTmuxREPL`'s wait path, and
> and no CI signal short of a live incident replay would catch it). cycle-1258's
> auditor WARN-shipped with a structured prescription to close exactly this gap
> — materialize this eval file, `git add -f` past `.gitignore` — and the
> prescription was never applied. `TestC1258_005_PermanentEvalEntryExistsAndItsEvidenceRuns`
> (`go/acs/cycle1258/predicates_test.go:318`) is the regression lock that reads
> this very file and executes its evidence commands; this eval's own evidence in
> turn re-runs the debounce unit tests directly, so a future regression in
> `artifactDetector`'s stability window fails both this cap and the unit suite
> it cites, not just a narrative claim.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| debounce regression undetected | stability-window debounce unit tests pass | 7/10 | `go test ./internal/bridge/... -run 'TestArtifactDetector_ReadyOnlyAfterCrossPollStability\|...'` |
| relocation races the debounce | fallback-relocation stability tests pass | 6/10 | `go test ./internal/bridge/... -run 'TestArtifactDetector_RelocationDeferredWhileFallbackStillGrowing\|...'` |
