---
score_cap:
  - criterion: "release-preflight fails with exit 1 and names a fixup! commit made since the last tag"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1890_001_FixupCommitSinceLastTagFailsNamingIt$' ./acs/cycle1890"
  - criterion: "Every WIP, fixup!, squash! and amend! subject since the last tag fails the preflight, and an untagged repo is scanned in full"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1890_00[23]_' ./acs/cycle1890"
  - criterion: "Clean commits, a WIP commit before the tag, and a prefix word mid-subject do not fail the preflight"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1890_004_CleanOrPreTagOrUnanchoredSubjectsPass$' ./acs/cycle1890"
  - criterion: "--require-ci fails on an unavailable CI verdict, passes on a green run, and the CI check stays advisory without the flag"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1890_00[567]_' ./acs/cycle1890"
  - criterion: "skills/release/SKILL.md calls release-preflight with --require-ci instead of gh run list and git log"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1890_008_ReleaseSkillDelegatesToPreflight$' ./acs/cycle1890"
---

# Eval: release-preflight refuses WIP commits and gains --require-ci

> Pins the readiness gate that moves from the `/evo:release` skill into `evolve release-preflight`.
> The preflight refuses any commit subject since the last tag that matches `^(WIP|fixup!|squash!|amend!)` and lists it.
> The `--require-ci` flag turns an unavailable remote CI verdict into a failure; without the flag the verdict stays advisory.
> The skill then calls the preflight instead of its own `gh run list` and `git log` steps.
> Source: inbox item cli-release-preflight-no-wip (2026-09-30), cycle 1890.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fixup-named | fixup! since the tag fails and is named | 8/10 | `TestC1890_001` |
| prefix-coverage | each WIP prefix fails; untagged repo scanned | 7/10 | `TestC1890_002`, `TestC1890_003` |
| no-overreach | clean, pre-tag, mid-subject pass | 7/10 | `TestC1890_004` |
| require-ci | fail closed only with the flag | 8/10 | `TestC1890_005..007` |
| skill-delegation | skill calls the preflight | 6/10 | `TestC1890_008` |
