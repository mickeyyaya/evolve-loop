---
score_cap:
  - criterion: "The cycle-1911 scoped-bench predicates pass when the host model catalog maps agy deep onto a Flash model"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1913_001 ./acs/cycle1913"
  - criterion: "The cycle-1911 scoped-bench predicates pass when the host model catalog maps agy deep onto Gemini 3.1 Pro"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1913_002 ./acs/cycle1913"
  - criterion: "All six cycle-1911 predicates pass with no host catalog and with a malformed host catalog"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1913_00[34]' ./acs/cycle1913"
---

# Eval: Make the cycle-1911 ACS predicates hermetic

> The cycle-1911 audit (defect H1, carryover `da04e697c7ea25783938c753349de87b7`) found that
> `TestC1911_003` depends on the host. The predicates read the live model catalog through
> `paths.ResolveFromEnv` and `bridge.LoadManifest`, so `EVOLVE_PROJECT_ROOT` selects the catalog.
> A host catalog that puts agy `deep` in a Flash group makes `agyTierGroups` fail.
> This eval pins the fix: the cycle-1911 predicates must give the same result under any host catalog.
> Source incident: cycle 1911 audit; reproduced in cycle 1913 by `TestC1913_001`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| deep-collapsed-onto-flash | 003 and 006 pass under a hostile host catalog | 8/10 | `go test -tags acs -run TestC1913_001 ./acs/cycle1913` |
| deep-on-gemini-pro | 003 and 006 pass under the host catalog that the audit saw | 6/10 | `go test -tags acs -run TestC1913_002 ./acs/cycle1913` |
| baseline-preservation | all six pass with no catalog and with a corrupt catalog | 7/10 | `go test -tags acs -run 'TestC1913_00[34]' ./acs/cycle1913` |
