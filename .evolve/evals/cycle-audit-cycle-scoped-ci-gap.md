---
score_cap:
  - criterion: "gofmt CI-parity gate is wired in audit.NewDefault so a gofmt-dirty worktree fails audit"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/phases/audit/... -run TestNewDefault_WiresGofmtCheck"
  - criterion: "SKILL.md-drift gate is wired in audit.NewDefault so drift fails audit before ship"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/phases/audit/... -run TestNewDefault_WiresSkillsDriftCheck"
---

# Eval: Audit gofmt + SKILL.md-drift gates wired (CI-parity)

> Pins that the cycle audit phase catches two classes of regression that caused
> cycles 339-341 to ship green-locally but red-in-CI:
> (1) a gofmt-dirty .go file in the cycle worktree (generated go/acs/cycle<N>/*.go
>     files were never formatted) → audit.NewDefault must wire gofmtCheckDefault;
> (2) SKILL.md phase-facts regions stale vs their SSOTs (profiles/registry edits
>     without regeneration) → audit.NewDefault must wire skillsDriftCheckDefault.
>
> Both fixes shipped in commits 23582c91 (gofmt gate, 2026-06-15) and 7feec764
> (skills-drift gate, 2026-06-15), motivated by the cycle-339/340/341 CI-red
> post-mortems. These evals ensure the gates are never accidentally un-wired.
>
> Source incident: cycles 339-341 shipped to main CI-RED; fixed forward in
> commit 965009c9; gates added to prevent recurrence (operator inbox item
> 2026-06-14T22:32:08Z, cycle-354 triage top_n).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| behavioral-gate | gofmt gate wired in NewDefault | 8/10 | `go test ./internal/phases/audit/... -run TestNewDefault_WiresGofmtCheck` |
| behavioral-gate | skills-drift gate wired in NewDefault | 8/10 | `go test ./internal/phases/audit/... -run TestNewDefault_WiresSkillsDriftCheck` |
