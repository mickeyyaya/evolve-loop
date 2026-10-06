---
score_cap:
  - criterion: "A malformed .evolve/policy.json yields a coded WARN on the phase-catalog path (evolve phases list) instead of a silent fallback to the default phase root"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1812_001_MalformedPolicyYieldsCodedWarnOnPhasesList ./acs/cycle1812"
  - criterion: "A missing or valid policy.json yields no policy warning, and a valid policy.json's declared phase root is honored"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1812_002_MissingOrValidPolicyYieldsNoPolicyWarn ./acs/cycle1812"
  - criterion: "TestValidateUserSpec fails when the reserved-kind rule (kind command) is removed from validate.go"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1812_003_ValidateUserSpecTestKillsReservedKindRuleMutant ./acs/cycle1812"
  - criterion: "Kind native validates clean with a valid name; kind command trips exactly the reserved-kind rule"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1812_004_ReservedKindIsCommandAndNativeIsExecutable ./acs/cycle1812"
  - criterion: "TestRepoPhaseCatalog_NoInertFailIfSignal reuses the shared eachTrackedPhaseClassify walk"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC1812_005_NoInertFailIfSignalVerdictDependsOnSharedWalk ./acs/cycle1812"
  - criterion: "phase-plugin-system.md §3.2 describes paths.phase_roots in .evolve/policy.json via phasespec.Roots, not EVOLVE_PHASE_ROOTS"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC1812_006_PluginDocSection32DescribesPolicyPhaseRoots ./acs/cycle1812"
  - criterion: "The phasespec package stays green"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestRoots|TestValidateUserSpec|TestRepoPhaseCatalog' ./internal/phasespec"
---

# Eval: phasespec Roots surfaces a malformed policy.json; the reserved-kind test case is no longer vacuous

> Pins the four defects reported by comment-reduction batch 34 (inbox item
> phasespec-roots-silent-policy-error, 2026-09-26) and fixed in cycle 1812.
> `phasespec.Roots` discarded the error from `policy.Load`, so a malformed
> `.evolve/policy.json` silently dropped operator-declared phase roots back to
> `.evolve/phases`. TestValidateUserSpec's "native reserved" case passed by
> accident: native is an executable kind, and only the single-word name "x"
> produced a violation that also contains "reserved", so removing the
> reserved-kind rule (kind command) killed no case. TestRepoPhaseCatalog_NoInertFailIfSignal
> duplicated the shared phase-catalog walk, and phase-plugin-system.md §3.2
> still described EVOLVE_PHASE_ROOTS, which no production code reads.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| silent-fallback-negative | malformed policy.json → coded WARN on `evolve phases list` | 7/10 | `TestC1812_001_*` |
| no-overfire-edge | missing / valid policy.json → no policy warning; declared root honored | 6/10 | `TestC1812_002_*` |
| mutation-kill | reserved-kind rule removal fails TestValidateUserSpec | 6/10 | `TestC1812_003_*` |
| semantic-anchor | native executable, command reserved | 5/10 | `TestC1812_004_*` |
| shared-walk | NoInertFailIfSignal verdict flows through eachTrackedPhaseClassify | 4/10 | `TestC1812_005_*` |
| doc-truth | §3.2 names paths.phase_roots / policy.json / phasespec.Roots | 4/10 | `TestC1812_006_*` |
| package-regression | phasespec suite green | 5/10 | `go test ./internal/phasespec` |
