---
score_cap:
  - criterion: "internal/rollback.Run, internal/pruneephemeral.Run and internal/marketplacepoll.Run each measure 50 lines or fewer under sizeratchet.Walk (inbox acceptance 3)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_00[123]_' ./acs/cycle1759/..."
  - criterion: "go/internal/sizeratchet/offenders.json keeps the three allowances 111/117/107 unchanged (inbox acceptance 3)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_004_OffendersJSONAllowancesUnchanged$' ./acs/cycle1759/..."
  - criterion: "The module-wide sizeratchet.Check stays green (inbox acceptance 3)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_010_ModuleWideRatchetCheckPasses$' ./acs/cycle1759/..."
  - criterion: "Behavior unchanged: the three packages' existing tests pass unmodified (inbox acceptance 1)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_005_TargetPackagesTestsPassUnmodified$' ./acs/cycle1759/..."
  - criterion: "No comment lines are added under the three shrunk packages (inbox acceptance 2)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_006_NoCommentsAddedToShrunkPackages$' ./acs/cycle1759/..."
  - criterion: "The cycle-1759 build explanation's before/after Run line counts equal the sizeratchet.Walk measurement (audit round 1 H1)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_007_ExplanationRunLineCountsMatchSizeratchet$' ./acs/cycle1759/..."
  - criterion: "The cycle-1759 build explanation calls no path committed that HEAD does not contain (audit round 1 H1)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_008_ExplanationCommitClaimsMatchGitHead$' ./acs/cycle1759/..."
  - criterion: "Every selected slug has a tracked, [code]-graded eval that grades only predicates the cycle defines (audit round 1 C1)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_009_EvalsMaterializedForSelectedSlugs$' ./acs/cycle1759/..."
---

# Eval: sizeratchet-shrink-run-commands

> Pins inbox item `2026-09-29T20-11-00Z-sizeratchet-shrink-run-commands.json`
> (fleet lane of the same id, cycle 1759): the three `Run` functions in
> `go/internal/rollback` (111 lines), `go/internal/pruneephemeral` (117) and
> `go/internal/marketplacepoll` (107) are shrunk to the `sizeratchet.MaxLines`
> (50) bar by extracting named steps, behavior unchanged, no comments added,
> and `offenders.json` left as slack for the boundary tighten. Scout split the
> item into `shrink-rollback-run`, `shrink-pruneephemeral-run` and
> `shrink-marketplacepoll-run`, each with its own eval.
>
> Source incident: cycle 1759 audit round 1 FAIL. C1: scout declared
> `evals-materialized` without writing any eval (Gate A parse-missed the
> `### Task N: <slug>` headings). H1: the build explanation claimed
> post-shrink sizes 27/24/24 against the measured 35/28/27, and called a
> staged file committed.

## Graders

### Size: all three Run functions fit the ratchet [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_00[123]_' ./acs/cycle1759/...
```

### Offenders ceiling untouched [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_004_OffendersJSONAllowancesUnchanged$' ./acs/cycle1759/...
```

### Module-wide ratchet green [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_010_ModuleWideRatchetCheckPasses$' ./acs/cycle1759/...
```

### Behavior unchanged [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_005_TargetPackagesTestsPassUnmodified$' ./acs/cycle1759/...
```

### Negative: no comment lines added [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_006_NoCommentsAddedToShrunkPackages$' ./acs/cycle1759/...
```

### Negative: explanation line counts match the measurement [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_007_ExplanationRunLineCountsMatchSizeratchet$' ./acs/cycle1759/...
```

### Negative: explanation commit claims match HEAD [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_008_ExplanationCommitClaimsMatchGitHead$' ./acs/cycle1759/...
```

### Evals materialized for every selected slug [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_009_EvalsMaterializedForSelectedSlugs$' ./acs/cycle1759/...
```

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | three Run functions <= 50 lines | 8/10 | `go test -run TestC1759_00[123]_` |
| offenders-ceiling | allowances 111/117/107 unchanged | 6/10 | `go test -run TestC1759_004...` |
| ratchet-gate | module-wide sizeratchet.Check green | 8/10 | `go test -run TestC1759_010...` |
| behavior-preserved | target package tests pass unmodified | 8/10 | `go test -run TestC1759_005...` |
| no-comments-added | commentaudit lists zero added lines | 7/10 | `go test -run TestC1759_006...` |
| narrative-counts | explanation before/after counts equal Walk | 6/10 | `go test -run TestC1759_007...` |
| narrative-commit | no committed claim for a path absent from HEAD | 5/10 | `go test -run TestC1759_008...` |
| evals-materialized | every selected slug has a graded, tracked eval | 7/10 | `go test -run TestC1759_009...` |
