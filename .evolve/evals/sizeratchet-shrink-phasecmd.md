---
score_cap:
  - criterion: "phasesCreate, phasesValidate and runPhaseVerify are each ≤50 lines"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1767_001_ThreeTargetFunctionsFitTheRatchetLimit ./acs/cycle1767"
  - criterion: "go/internal/sizeratchet/offenders.json is untouched (allowances stay as unclaimed slack)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1767_002_OffendersJSONLeftUnchanged ./acs/cycle1767"
  - criterion: "the module-wide size ratchet stays green after the shrink"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1767_003_ModuleWideRatchetCheckPasses ./acs/cycle1767"
  - criterion: "existing go/internal/cli/phasecmd tests pass unmodified (behavior preserved)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/cli/phasecmd"
  - criterion: "no comment lines were added to the touched phasecmd files"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1767_005_NoCommentLinesAdded ./acs/cycle1767"
---

# Eval: Shrink phasecmd size-ratchet offenders

> Pins the cycle-1767 Extract Method refactor of `runPhaseVerify`
> (`phase_verify.go`), `phasesValidate` (`phases.go`) and `phasesCreate`
> (`phases_create.go`) in `go/internal/cli/phasecmd`, each currently over the
> 50-line size-ratchet limit (74/69/123 lines) per
> `go/internal/sizeratchet/offenders.json`. The refactor must be
> behavior-preserving (existing test suite unmodified and green), must add no
> comments (naming carries intent per docs/conventions/code-comments.md), and
> must leave `offenders.json` untouched — the allowance is a ceiling only, so a
> shrunk function's entry becomes unclaimed slack for a later boundary tighten
> rather than something this lane edits itself.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ratchet-limit | All 3 target functions ≤50 lines | 8/10 | `go test -tags acs -run TestC1767_001_...` |
| offenders-untouched | offenders.json byte-identical to baseline, same 3 allowances | 7/10 | `go test -tags acs -run TestC1767_002_...` |
| module-ratchet-green | Whole-module sizeratchet.Check passes | 6/10 | `go test -tags acs -run TestC1767_003_...` |
| behavior-preserved | go/internal/cli/phasecmd test suite passes unmodified | 8/10 | `go test ./internal/cli/phasecmd` |
| no-comments-added | commentaudit reports zero added comment lines under phasecmd | 5/10 | `go test -tags acs -run TestC1767_005_...` |
