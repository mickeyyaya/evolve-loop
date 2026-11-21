---
score_cap:
  - criterion: "agy.json declares no model_tier_map and points at the agy-tmux family table"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2580_001' ./acs/cycle2580"
  - criterion: "LoadManifest(agy) returns the same tier map as the agy-tmux family manifest"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2580_002' ./acs/cycle2580 && go test -count=1 -run '^TestAgyHeadlessManifest_PointsAtFamilyTable$' ./internal/bridge"
  - criterion: "The profiles and setup packages and the agy consumers in bridge stay green"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2580_00[456]' ./acs/cycle2580"
---

# Eval: agy headless manifest adopts the family tier map

> Pins the single declaration of the agy model tier map in agy-tmux.json. The headless agy.json carried a second copy that disagreed with the family table, as codex.json did before the codex-tier-map-single-source inbox item. Source: inbox item agy-tier-map-single-source, cycle 2580.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| single-declaration | agy.json holds no table and names agy-tmux | 5/10 | `TestC2580_001` |
| relational-equality | loaded maps are equal, no literal ids | 6/10 | `TestC2580_002`, `TestAgyHeadlessManifest_PointsAtFamilyTable` |
| consumers-green | profiles, setup, bridge agy consumers stay green | 6/10 | `TestC2580_004`, `TestC2580_005`, `TestC2580_006` |
