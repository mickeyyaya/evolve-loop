---
score_cap:
  - criterion: "ValidateUserSpec rejects single-word user phase names with a multi-word violation"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestBugRepro_Cycle229_TwoTierNamingMissing ./internal/phasespec/"
  - criterion: "ValidateUserSpec still accepts multi-word kebab-case names and rejects malformed ones"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestTwoTierNaming_MultiWordAccepted|TestTwoTierNaming_MalformedRejected' ./internal/phasespec/"
  - criterion: "Cycle-229 red anchor test is git-tracked"
    max_if_missing: 7
    evidence: "git ls-files --error-unmatch go/internal/phasespec/bug_repro_cycle229_test.go"
---

# Eval: Two-tier naming gate in ValidateUserSpec

> Pins the two-tier phase naming rule (0149d81: single-word names are a closed
> builtin vocabulary; user/optional/minted phases must be multi-word
> `<object>-<action>` kebab-case) at the authoring gate, so the advisor's
> plan-time clamp can trust the registry as clean input. Source incident:
> cycle 229 placed `bug_repro_cycle229_test.go` as a red anchor —
> `ValidateUserSpec` accepted `"scanner"` because only the format-safety
> `nameRE` ran; the worktree fix was never shipped (regression-gate FAIL).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| single-word-rejected | red anchor test passes | 4/10 | `go test -run TestBugRepro_Cycle229_TwoTierNamingMissing ./internal/phasespec/` |
| no-over-restriction | multi-word accepted, malformed rejected | 6/10 | `go test -run 'TestTwoTierNaming_MultiWordAccepted\|TestTwoTierNaming_MalformedRejected' ./internal/phasespec/` |
| anchor-tracked | TDD anchor committed with the fix | 7/10 | `git ls-files --error-unmatch go/internal/phasespec/bug_repro_cycle229_test.go` |
