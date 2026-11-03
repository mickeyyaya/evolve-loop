---
score_cap:
  - criterion: "A fleet-mode advisor input with an empty ActiveWorktree is refused before any launch"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2136_001_FleetAdvisorInputWithEmptyActiveWorktreeIsRefused ./acs/cycle2136"
  - criterion: "The advisor package holds a passing fleet worktree test"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2136_005_AdvisorPackageHoldsAFleetWorktreeTestThatPasses ./acs/cycle2136"
  - criterion: "Single mode keeps the workspace fallback for an empty ActiveWorktree"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2136_004_NonFleetEmptyActiveWorktreeKeepsTheWorkspaceFallback ./acs/cycle2136"
---

# Eval: Pin the fleet-mode ActiveWorktree requirement

> Pins the invariant in docs/architecture/packages/internal-router.md that a fleet-mode caller sets RouteInput.ActiveWorktree. Before cycle 2136 the advisor replaced an empty value with the workspace, so the bridge fleet refusal never fired. Source incident: inbox item router-active-worktree-fleet-pin-console (cycle 2000 routed the lane part to the console).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fleet-refusal | Empty fleet worktree is refused before launch | 6/10 | `TestC2136_001` |
| pinned-in-package | Advisor package holds a passing fleet worktree test | 7/10 | `TestC2136_005` |
| single-mode-preserved | Workspace fallback stays outside fleet mode | 6/10 | `TestC2136_004` |
