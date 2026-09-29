---
score_cap:
  - criterion: "cmd/evolve.runResetSHA in go/cmd/evolve/cmd_resetsha.go is at or under the 50-line size-ratchet ceiling (was 51) and keeps its name"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1754_001_RunResetSHAWithinSizeRatchetLimit ./acs/cycle1754"
  - criterion: "the module-wide size ratchet passes: no extracted helper is past 50 lines and no listed function is past its offenders.json allowance (the runResetSHA entry is either removed or left as slack)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1754_003_ModuleSizeRatchetCheckPasses ./acs/cycle1754"
  - criterion: "reset-sha root resolution is behavior-preserved: flag beats EVOLVE_PROJECT_ROOT beats cwd, a relative flag or env root resolves against cwd, a missing relative root fails exit 1 naming the absolute state path, and runResetSHA no longer resolves the root inline"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1754_004_ResetSHARootResolutionBehaviorPreserved ./acs/cycle1754"
  - criterion: "the touched cmd/evolve files are gofmt clean and go vet ./cmd/evolve/ exits 0"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1754_007_TouchedFilesGofmtAndVetClean ./acs/cycle1754"
---

# Eval: Shrink cmd/evolve.runResetSHA by extracting its project-root resolution

> Pins the cycle-1754 size-ratchet shrink of `runResetSHA` (go/cmd/evolve/cmd_resetsha.go,
> 51 lines, listed at 51 in go/internal/sizeratchet/offenders.json). The flag/env/cwd
> fallback and the `paths.AbsoluteRoot` call move into a helper; the function must fit
> 50 lines without a rename, and the characterization tests written first (the command
> had only two tests, both on the explicit flag) must pass unchanged, including the
> negative case that a missing relative root reaches `phaseintegrity.RepinShipSHA` as an
> absolute path and fails there. Source: cycle 1754 scout/triage top_n.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| size-ceiling | runResetSHA <= 50 lines, name kept | 8/10 | `TestC1754_001_RunResetSHAWithinSizeRatchetLimit` |
| ratchet-clean | module-wide `sizeratchet.Check` passes | 7/10 | `TestC1754_003_ModuleSizeRatchetCheckPasses` |
| behavior+negative | precedence, relative roots, missing-root refusal; no inline resolution | 9/10 | `TestC1754_004_ResetSHARootResolutionBehaviorPreserved` |
| build-clean | gofmt + go vet | 6/10 | `TestC1754_007_TouchedFilesGofmtAndVetClean` |
