---
score_cap:
  - criterion: "The raw git fixture ratchet (default untagged suite of go/internal/rawgitratchet) passes on the current tree, and its list of existing call sites, go/internal/rawgitratchet/baseline.json, is git-tracked"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1729_001_' ./acs/cycle1729/"
  - criterion: "A new raw git init in a test outside internal/gittest fails the ratchet and names the file: literal exec.Command shape, helper-wrapped shape, outside internal/ (cmd/evolve), and one more call site inside an already-listed file; a raw init inside internal/gittest stays allowed"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1729_00[23458]_' ./acs/cycle1729/"
  - criterion: "The existing call sites are listed and may only shrink: a migrated file must leave the list, and migrating one site frees no slot for a new one"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1729_00[67]_' ./acs/cycle1729/"
  - criterion: "The ratchet binds git-tracked test files only (ADR-0084 I1): an untracked raw-init file does not red it until staged, and a module outside any git work tree binds every on-disk test file instead of none"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1729_0(09|10)_' ./acs/cycle1729/"
  - criterion: "The ship-time repo-contract scanner pack (ship.repoContractPackages) reaches the ratchet and reds on a new staged raw init as a named test failure; the package is apicover-clean and enrolled in go/.apicover-enforce"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1729_01[12]_' ./acs/cycle1729/"
---

# Eval: A ratchet refuses a new raw git fixture outside internal/gittest

> Pins inbox item `raw-git-fixture-ratchet` (design doc §7.9 G3). Cycle 1706
> moved the three tests it had seen into `internal/gittest`, fixing the flake
> where a detached git maintenance child races `t.TempDir()` removal. About 116
> test files still build raw git repositories, and nothing stopped a new one.
> The acceptance: a repo-contract test fails on a new raw git init in a test
> outside `internal/gittest`, and the existing call sites are listed and may
> only shrink.
>
> Source incidents: cycle 1706 (the instance fix); cycle 1725, whose
> implementation of this item was audited WARN (M1: the scanner walked
> untracked on-disk files, an ADR-0084 I1 violation) and never shipped
> (GIT_FLEET_REBASE_NEEDED). Criterion 4 carries that audit's lesson into this
> retry. Every negative predicate runs the real ratchet against a throwaway
> copy of the module, so the evidence is safe to replay in later cycles.
> 005–007 skip once no literal raw-init site is left to migrate.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exists-and-green | ratchet package present, untagged, green on the tree; baseline.json tracked | 6/10 | `TestC1729_001` |
| new-site-refused | literal, helper-wrapped, cmd/ and in-listed-file raw inits each fail, named; gittest exempt | 8/10 | `TestC1729_002`–`005`, `008` |
| list-only-shrinks | a stale entry fails; a migration does not launder a new site | 7/10 | `TestC1729_006`, `TestC1729_007` |
| tracked-state-binding | untracked never reds; staged does; non-git module binds everything | 7/10 | `TestC1729_009`, `TestC1729_010` |
| ship-reachability + apicover | the pack reaches the ratchet as a named test fail; the package's exports are named, executed and documented | 8/10 | `TestC1729_011`, `TestC1729_012` |
