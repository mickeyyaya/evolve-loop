---
score_cap:
  - criterion: "each of the three TESTHOST_REQUIRED_TOOLS lines of go.yml declares git, so a runner without git fails the test"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2541_001_EachTestJobDeclaresGitRequired ./acs/cycle2541"
  - criterion: "each declaration stays well formed and keeps tmux"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2541_002_EachDeclarationStaysWellFormedAndKeepsTmux ./acs/cycle2541"
  - criterion: "the tmux tier claim of the workflow still holds, so CI stays green"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier ./internal/ciparity"
---

# Eval: git required on the three Go test jobs of CI

> Pins task T3 of the test-quality audit (docs/plans/test-quality-audit-2026-11.md). A job that declares git in TESTHOST_REQUIRED_TOOLS turns a missing git into a failed test through testhost.Tool, and it never skips in silence. Source incident: cycle 2541.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| git-declared | the three lines list git | 6/10 | `go test -tags acs -run TestC2541_001_EachTestJobDeclaresGitRequired ./acs/cycle2541` |
| well-formed | no empty or repeated entry, tmux kept | 7/10 | `go test -tags acs -run TestC2541_002_EachDeclarationStaysWellFormedAndKeepsTmux ./acs/cycle2541` |
| tier-claim | the ciparity tmux claim holds | 7/10 | `go test -run TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier ./internal/ciparity` |
