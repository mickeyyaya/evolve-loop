---
score_cap:
  - criterion: "UpstreamDigest returns a hard rune-capped, non-fabricated projection of sealed Handoffs"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1604_001_UpstreamDigestBoundedByCap|TestC1604_002_UpstreamDigestAbsentForZeroValueHandoffs' ./acs/cycle1604"
  - criterion: "Degraded read-misses surface in the digest instead of collapsing into clean absence (R5) — including when an oversized typed view renders before them"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1604_003_UpstreamDigestSurfacesDegradedReads ./acs/cycle1604"
  - criterion: "No single oversized section can evict another: every present section survives an oversized scalar placed in scout, triage, generic or degraded, under an explicit cap and the package default"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1632_009_UpstreamDigestOversizedScalarCannotEvictOtherSections ./acs/cycle1632"
  - criterion: "The digest is deterministic across repeated calls on identical input (no map-iteration-order leak)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1604_004_UpstreamDigestDeterministicAcrossCalls ./acs/cycle1604"
  - criterion: "Present-but-zero-value upstream views render distinctly from genuinely absent ones"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1604_005_UpstreamDigestDistinguishesPresentZeroFromAbsent ./acs/cycle1604"
---

# Eval: bounded upstream handoff digest contract (phaseio.Handoffs.UpstreamDigest)

> Pins the `phaseio.Handoffs.UpstreamDigest(capRunes int) string` contract
> reintroduced in cycle 1604 for the fleet-lane inbox item
> `tokenopt-handoff-digests`. A cycle-1603 audit artifact claimed this exact
> contract had already landed (`../cycle-1603/audit-report.md#Handoff
> Summary`), but the live worktree at cycle 1604 had neither `digest.go` nor
> this eval file — a cross-worktree audit/tree mismatch (scout-report.md Key
> Finding 2). This eval makes the contract permanent so a future cycle cannot
> silently regress it the same way.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| hard-cap | never exceeds requested rune cap; non-empty when data present | 7/10 | `TestC1604_001` |
| absence-fidelity | zero-value Handoffs → empty digest, never fabricated | 7/10 | `TestC1604_002` |
| degraded-visibility | R5 read-misses named, not dropped — also behind an oversized ScoutView | 7/10 | `TestC1604_003` (two fixtures since cycle 1632) |
| section-budget | an oversized scalar in one section never evicts another (cycle-1632 audit M1; cycle-1604 df875c36…/d69bd9f3…) | 8/10 | `TestC1632_009` |
| determinism | identical input → identical output across calls | 6/10 | `TestC1604_004` |
| present-zero-vs-absent | a present-but-empty view still renders its section | 6/10 | `TestC1604_005` |
