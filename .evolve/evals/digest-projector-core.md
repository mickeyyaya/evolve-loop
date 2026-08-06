---
score_cap:
  - criterion: "ProjectDigest extracts only the requested role's tagged sections — no untagged/other-role leakage"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1391_001_ProjectDigestExtractsOnlyTaggedRoleSections ./acs/cycle1391/"
  - criterion: "ProjectDigest achieves >= 50% byte reduction on a fixture with a large excluded block"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1391_002_ProjectDigestByteReductionAtLeastHalf ./acs/cycle1391/"
  - criterion: "a role with zero matching marker blocks gets an empty digest, never a silent full-source fallback"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1391_003_ProjectDigestRoleWithNoMatchIsEmptyNotFullSource ./acs/cycle1391/"
  - criterion: "an unterminated digest marker is rejected with a non-nil error, not silently truncated"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1391_004_ProjectDigestUnterminatedMarkerErrors ./acs/cycle1391/"
---

# Eval: SSOT digest projector — core parser (go/internal/digest)

> Pins the behavioral contract of `digest.ProjectDigest(source []byte, role
> string) ([]byte, error)`, the cycle-1391 first increment of inbox item
> `tokenopt-role-scoped-instruction-digests`. Full phase-instruction skill
> files (`skills/loop/evolve-scout.md` etc.) are injected verbatim per phase
> per cycle even though narrow-role phases (e.g. scout) never act on most of
> the cross-cutting ship-gate/audit content in them — the inbox item frames
> this as a ~24x per-cycle token cost, and `docs/research/llm-output-stability-2026-07/`
> reframes it as a quality risk too (context-rot degrades accuracy well
> before the context limit). The projector parses
> `<!-- digest:role=ROLE[,ROLE2,...] -->...<!-- /digest -->` marker pairs out
> of an SSOT source and returns only the blocks tagged for the requested
> role — a single reusable projector, not a hand-maintained per-role copy
> (`skills/fable/SKILL.md:143` independently wanted the same pattern). The
> no-match-is-empty cap exists because a no-op/pass-through implementation
> would trivially satisfy an extraction test that only checks "contains the
> right content" while silently returning the entire source for every role —
> the negative case forces genuine projection. Source: scout-report.md
> cycle 1391, Task 1.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| role-isolation | own-role content present, other-role/untagged content absent | 8/10 | `TestC1391_001_...` |
| size-reduction | digest < 50% of source on an excluded-heavy fixture | 7/10 | `TestC1391_002_...` |
| anti-no-op | unmatched role → empty digest, not full source | 8/10 | `TestC1391_003_...` |
| malformed-input | unterminated marker → non-nil error | 6/10 | `TestC1391_004_...` |
