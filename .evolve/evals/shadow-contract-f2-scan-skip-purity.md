---
score_cap:
  - criterion: "regressiontia.Pure proves only hatch-free closures and fails closed"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestPure_' ./internal/regressiontia"
  - criterion: "the scan skip is default-deny: env readers, git runners and unlisted impure packages are uncacheable"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestCacheable_' ./internal/phases/ship"
  - criterion: "an ambient environment change never hits the stored green of an env-reading package"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestScanShadow_AmbientEnvChangeDoesNotHit' ./internal/phases/ship"
  - criterion: "the scorer counts a false stop, an escape and a conflict"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestScorer' ./internal/stagescore"
---

# Eval: Ship scan skip, one default-deny purity predicate

> Pins the F2 fix of the shadow-stage contract. The scan skip escaped twice (internal/acssuite, cycle 2362; internal/guards, 2026-11-13) because its whole-tree content key misses the environment, `.git` and subprocess inputs of impure packages. Cacheability now requires `regressiontia.Pure` (default-deny), the hand list stays an override, and the scorer counts false stops, escapes and conflicts. Source incident: cycle 2420.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| purity-predicate | Pure proves only hatch-free closures | 5/10 | `go test -run TestPure_ ./internal/regressiontia` |
| default-deny | Impure or unlisted packages are uncacheable | 4/10 | `go test -run TestCacheable_ ./internal/phases/ship` |
| ambient-env | An env change never hits a stored green | 4/10 | `go test -run TestScanShadow_AmbientEnvChangeDoesNotHit ./internal/phases/ship` |
| scorer | False stop, escape and conflict are counted | 6/10 | `go test -run TestScorer ./internal/stagescore` |
