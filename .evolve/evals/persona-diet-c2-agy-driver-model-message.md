---
score_cap:
  - criterion: "The agy headless driver log and argv are pinned by driver tests"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestAgyDriver_Message|TestAgyDriver_HeadlessArgv' ./internal/bridge"
  - criterion: "No agy driver message says agy has no model flag"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestAgyDriver_NoMessageSaysAgyHasNoModelFlag ./internal/bridge"
  - criterion: "The model and the effort come from the route data and a route change changes the argv"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestAgyDriver_RouteChangeChangesTheArgv|TestAgyDriver_RouteIntentFromConfigReachesTheArgv' ./internal/bridge"
---

# Eval: The agy driver states the real agy flags and takes the model and effort from the route data

> The headless agy driver said that agy has no `-m` flag and that agy ignores a model. agy 1.5.17 has `--model` and `--effort`. These tests pin the log text and the headless argv, and prove that the manifest route data (`model_tier_map`, effort) changes the argv. Source: cycle 2679, plan C2 in `docs/plans/persona-diet-2026-11.md`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| pinned-text-and-argv | Message and argv are pinned | 6/10 | `go test -run 'TestAgyDriver_Message\|TestAgyDriver_HeadlessArgv'` |
| stale-message-absent | No "no model flag" message | 7/10 | `go test -run TestAgyDriver_NoMessageSaysAgyHasNoModelFlag` |
| route-driven-argv | Route data drives model and effort | 7/10 | `go test -run TestAgyDriver_RouteChangeChangesTheArgv` |
