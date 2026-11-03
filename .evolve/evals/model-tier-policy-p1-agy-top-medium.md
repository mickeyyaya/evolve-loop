---
score_cap:
  - criterion: "The agy-tmux manifest maps top to Gemini 4 Argon (Medium)"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2134_00[12]_' ./acs/cycle2134"
  - criterion: "The bridge and setup pin tests pass and pin Medium at top even when the live catalog maps agy top to Argon (High)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2134_00[45]_' ./acs/cycle2134"
  - criterion: "The plan marks P1 built and leaves P2 open"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2134_006_' ./acs/cycle2134"
---

# Eval: Model tier P1, the agy top tier names Gemini 4 Argon (Medium)

> Pins the P1 component of docs/plans/model-tier-policy-2026-10.md (decision D4). The agy manifest maps `top` to Argon (Medium), and the two tests that pin the tier map follow. The pin tests read the manifest through LoadManifest, which overlays the live catalog, so they must isolate the catalog. Source incident: cycle 2134 audit round 1 (ACS red=4 in the live root), inbox item model-tier-policy-p1-agy-top-medium.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| manifest-value | top loads as Argon (Medium), never Argon (High) | 4/10 | `go test -tags acs -run TestC2134_00[12]_ ./acs/cycle2134` |
| pin-tests | bridge and setup pin tests pass and pin Medium under a hostile live catalog | 6/10 | `go test -tags acs -run TestC2134_00[45]_ ./acs/cycle2134` |
| plan-status | P1 row marked Built, P2 row still Open | 7/10 | `go test -tags acs -run TestC2134_006_ ./acs/cycle2134` |
