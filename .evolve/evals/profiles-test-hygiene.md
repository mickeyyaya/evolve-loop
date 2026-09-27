---
score_cap:
  - criterion: "TestEveryAgentProfileHasAFallbackChain passes with an untracked stub profile (cli set, no cli_fallback) present in .evolve/profiles"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1718_001_' ./acs/cycle1718/"
  - criterion: "The fallback-chain test still fails on a TRACKED profile with no cli_fallback, and binds every on-disk profile when git context is absent"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1718_00[23]_' ./acs/cycle1718/"
  - criterion: "Exactly one profiles-dir helper remains in go/internal/profiles (one *ProfilesDir(t) func, one runtime.Caller .evolve resolver) and the effort matrix still reads the live profiles through it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1718_004_' ./acs/cycle1718/"
  - criterion: "The router-only loop-unblock test is renamed to state what it asserts (Router + Agy + Fallback) and still fails when the router leaves agy-tmux or its claude-tmux fallback"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1718_005_' ./acs/cycle1718/"
  - criterion: "rescue-branch-disposition-2026-06-07.md no longer claims main leaves allowed_tools unexpanded, and its expandPolicies(AllowedTools) row names Get"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1718_006_' ./acs/cycle1718/"
  - criterion: "The profiles package suite stays green"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/profiles/"
---

# Eval: profiles test hygiene — tracked-set fallback check, one dir helper, honest names

> Pins `profiles-test-hygiene` (inbox 2026-09-26, test-hygiene, P3, weight 0.35;
> found by the comment-reduction workstream, batch 2). The defects:
> `TestEveryAgentProfileHasAFallbackChain` walked `.evolve/profiles` with
> `os.ReadDir`, so the runtime's untracked stubs can red it on a busy plane (the
> `cd49274beab2` false-RED class that `RealTreeProfiles` already closes for the
> other real-tree scans). `effortProfilesDir` and `trackedProfilesDir` duplicated
> `realProfilesDir`. `TestLoopUnblockProfilesRouteTimeoutPronePhasesToAgy`
> checks only the router. The rescue-branch disposition doc said main leaves
> `allowed_tools` unexpanded, but `(*Loader).Get` now expands it. Cycle 1718's RED
> run reproduced the first defect by running the real test binary over a mirror
> of the tracked profiles plus one untracked stub. The unfixed test failed with
> `zz-untracked-stub-c1718.json: cli=claude-tmux has NO cli_fallback`.
> Source incident: cycle 1718 TDD (`go/acs/cycle1718/predicates_test.go`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| untracked-stub-tolerated | fallback-chain test ignores untracked runtime-minted stubs | 7/10 | `TestC1718_001` |
| tracked-violator-still-red | filter skips only untracked stubs; bind-all without git | 7/10 | `TestC1718_002`, `TestC1718_003` |
| one-dir-helper | single `*ProfilesDir` / runtime.Caller resolver | 6/10 | `TestC1718_004` |
| honest-router-test-name | renamed test still pins router CLI + fallback | 6/10 | `TestC1718_005` |
| doc-matches-code | doc row states main's `Get` expands `allowed_tools` | 4/10 | `TestC1718_006` |
| no-regression | `./internal/profiles` suite green | 5/10 | `go test -count=1 ./internal/profiles/` |
