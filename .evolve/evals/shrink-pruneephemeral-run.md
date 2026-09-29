---
score_cap:
  - criterion: "internal/pruneephemeral.Run measures 50 lines or fewer under sizeratchet.Walk (was 117)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_002_PruneephemeralRunFitsSizeRatchet$' ./acs/cycle1759/..."
  - criterion: "The module-wide sizeratchet.Check stays green over Walk(go) and the unchanged offenders.json"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_010_ModuleWideRatchetCheckPasses$' ./acs/cycle1759/..."
  - criterion: "go/internal/sizeratchet/offenders.json keeps the internal/pruneephemeral.Run allowance at 117 (an allowance is a ceiling; this lane does not edit it)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_004_OffendersJSONAllowancesUnchanged$' ./acs/cycle1759/..."
  - criterion: "Behavior unchanged: the target packages' existing tests pass unmodified"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_005_TargetPackagesTestsPassUnmodified$' ./acs/cycle1759/..."
  - criterion: "No comment lines are added under the shrunk packages (commentaudit comments -base b401e73d)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_006_NoCommentsAddedToShrunkPackages$' ./acs/cycle1759/..."
  - criterion: "The cycle-1759 build explanation states the before/after line count of internal/pruneephemeral.Run that sizeratchet.Walk measures"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1759_007_ExplanationRunLineCountsMatchSizeratchet$' ./acs/cycle1759/..."
---

# Eval: shrink-pruneephemeral-run

> Pins the extraction refactor of `internal/pruneephemeral.Run` (`go/internal/pruneephemeral/pruneephemeral.go`, 117 lines at baseline
> b401e73d) down to the `sizeratchet.MaxLines` (50) bar with behavior
> unchanged, no comments added, and `offenders.json` left as slack for the
> boundary tighten. Scout slug of inbox item
> `2026-09-29T20-11-00Z-sizeratchet-shrink-run-commands.json`, fleet lane
> `sizeratchet-shrink-run-commands`, cycle 1759.
>
> Source incident: cycle 1759 audit round 1 failed on C1 (scout declared
> `evals-materialized` but wrote no eval for this slug; Gate A parse-missed
> the `### Task N: <slug>` headings) and H1 (the build explanation claimed
> post-shrink sizes 27/24/24 while `sizeratchet.Walk` measured 35/28/27).

## Graders

### Size: internal/pruneephemeral.Run fits the ratchet [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_002_PruneephemeralRunFitsSizeRatchet$' ./acs/cycle1759/...
```

### Module-wide ratchet green [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_010_ModuleWideRatchetCheckPasses$' ./acs/cycle1759/...
```

### Offenders ceiling untouched [code]
```bash
cd go && go test -tags acs -count=1 -run '^TestC1759_004_OffendersJSONAllowancesUnchanged$' ./acs/cycle1759/...
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

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | internal/pruneephemeral.Run <= 50 lines | 8/10 | `go test -run TestC1759_002_PruneephemeralRunFitsSizeRatchet` |
| ratchet-gate | module-wide sizeratchet.Check green | 8/10 | `go test -run TestC1759_010...` |
| offenders-ceiling | allowance 117 unchanged | 6/10 | `go test -run TestC1759_004...` |
| behavior-preserved | target package tests pass unmodified | 8/10 | `go test -run TestC1759_005...` |
| no-comments-added | commentaudit lists zero added lines | 7/10 | `go test -run TestC1759_006...` |
| narrative-truth | explanation before/after counts equal Walk | 6/10 | `go test -run TestC1759_007...` |
