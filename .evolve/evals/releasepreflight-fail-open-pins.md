---
score_cap:
  - criterion: "A permission error or any other non-not-exist stat error on .evolve/naming.json fails defaultNameGuard instead of passing as an absent manifest"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestDefaultNameGuard_StatErrorsOtherThanNotExistFail$' ./internal/releasepreflight"
  - criterion: "A missing naming manifest stays a clean pass, pinned by TestDefaultNameGuard_MissingManifestIsCleanPass"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestDefaultNameGuard_MissingManifestIsCleanPass$' ./internal/releasepreflight"
  - criterion: "A CI lookup failure is the unavailable verdict and never an error, pinned by TestDefaultCIConclusion_LookupFailuresAreUnavailableNotErrors"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestDefaultCIConclusion_LookupFailuresAreUnavailableNotErrors$' ./internal/releasepreflight"
  - criterion: "The simulation-suite failure warning reads '(advisory)' with no stale v12.1.5/v12.2.0 roadmap text"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestRun_SimulationAdvisory$' ./internal/releasepreflight"
---

# Eval: releasepreflight fail-open rules are pinned by tests, and an unreadable naming manifest fails

> Pins the two fail-open rules of release preflight and the boundary between them
> and a real failure. `defaultNameGuard` read every `os.Stat` error on
> `.evolve/naming.json` (EACCES, ELOOP) as "no manifest" and passed step 5; only
> not-exist may pass. `defaultCIConclusion` turns every lookup failure (no git
> repository, git or gh absent, gh exit, malformed or empty JSON) into the
> unavailable verdict with a nil error. Before this change, two why comments held
> these rules and no test did. The simulation warning also logged a stale
> "advisory in v12.1.5; required in v12.2.0" roadmap marker. Source: inbox item
> releasepreflight-fail-open-pins (PR #749 comment round 12), cycle 1833.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| stat-error-boundary | A non-not-exist stat error fails the naming guard | 8/10 | `go test -run '^TestDefaultNameGuard_StatErrorsOtherThanNotExistFail$'` |
| missing-manifest-pin | A missing manifest is a clean pass | 6/10 | `go test -run '^TestDefaultNameGuard_MissingManifestIsCleanPass$'` |
| ci-unavailable-pin | A CI lookup failure is unavailable, not an error | 6/10 | `go test -run '^TestDefaultCIConclusion_LookupFailuresAreUnavailableNotErrors$'` |
| stale-log-text | The simulation warning reads "(advisory)" | 5/10 | `go test -run '^TestRun_SimulationAdvisory$'` |
