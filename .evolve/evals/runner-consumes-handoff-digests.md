---
score_cap:
  - criterion: "The real tdd ComposePrompt entry point renders the upstream digest reachable from the production PhaseInput channel"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1604_006_TDDComposePromptRendersUpstreamDigestFromRealCaller ./acs/cycle1604"
  - criterion: "The rendered live prompt never contains an oversized raw upstream value verbatim (cap governs the real caller, not just the unit)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1604_007_TDDComposePromptCapsDigestNoRawArtifactLeak ./acs/cycle1604"
  - criterion: "An inactive PhaseInput leaves the tdd prompt byte-identical to the legacy (no-digest) rendering"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1604_008_TDDComposePromptByteIdenticalWhenInactive ./acs/cycle1604"
---

# Eval: live TDD composer consumes the bounded upstream handoff digest

> Pins the wiring proof for `tokenopt-handoff-digests`: a digest producer with
> no live consumer is inert. This eval requires the REAL exported
> `tdd.New(...).ComposePrompt` (never a predicate calling `phaseio` directly,
> and never the unexported `hooks` type) to render the digest when
> `req.Input.Active()`, cap it against a raw-artifact leak on that same live
> path, and leave the inactive/legacy path byte-identical. Re-authored in
> cycle 1604 after a cycle-1603 audit artifact claimed this wiring existed
> (`../cycle-1603/audit-report.md#Handoff Summary`) while the live worktree at
> cycle 1604 had no `.evolve/evals/runner-consumes-handoff-digests.md` and no
> consumer hunk in `tdd.go` — see scout-report.md Key Finding 1/2.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| real-caller-proof | production `ComposePrompt` renders digest content from a real `PhaseInput` | 8/10 | `TestC1604_006` |
| anti-leak | oversized raw upstream value never appears verbatim in the rendered prompt | 8/10 | `TestC1604_007` |
| byte-identical-inactive | inactive envelope leaves the legacy prompt untouched | 6/10 | `TestC1604_008` |
