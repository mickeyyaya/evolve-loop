---
score_cap:
  - criterion: "`evolve audit calibration` is dispatchable through the real CLI registry with explicit --dossiers-dir/--runs-dir/--output paths and is documented in `evolve help`"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run TestC1648_001_AuditCalibrationIsReachableThroughTheRealCLI ./acs/cycle1648/"
  - criterion: "The report keeps narrative, chain, deterministic-gate, shipped and override outcomes as distinct columns and its (narrative x gate) matrix counts the fixture cells exactly"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run TestC1648_002_MatrixPreservesNarrativeGateShippedAndChainDistinctly ./acs/cycle1648/"
  - criterion: "A narrative PASS force-overridden to FAIL is listed as an override and lands in the (PASS,FAIL) disagreement cell, never in (PASS,PASS); a control corpus without the override renders differently"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags acs -run TestC1648_003_ForceOverriddenNarrativePassIsNeverCollapsedIntoAgreement ./acs/cycle1648/"
  - criterion: "Missing/non-directory corpus roots, a missing --output, an unwritable output directory, and a missing/unknown subcommand each exit non-zero, name the offender on stderr, and write no report"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run TestC1648_004_InvalidCorpusInputsFailLoudly ./acs/cycle1648/"
  - criterion: "Missing-shadow, malformed-shadow, missing-dossier, malformed-dossier and malformed-fail-reason cycles are counted and listed as named exclusions and never enter the matrix; an empty corpus renders zeros"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags acs -run TestC1648_005_MalformedAndMissingPairsAreCountedAsExclusionsNeverAgreement ./acs/cycle1648/"
  - criterion: "Two runs over the same corpus are byte-identical, carry no timestamp, and list pairs/exclusions in ascending cycle order"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run TestC1648_006_ReportIsDeterministicAcrossRuns ./acs/cycle1648/"
  - criterion: "audit-fail-reason.json reasons are normalised to defect classes (text before the first colon) and aggregated across cycles; a pair without a reason artifact has no classes"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run TestC1648_007_DefectClassBreakdownCountsNormalizedReasonClasses ./acs/cycle1648/"
  - criterion: "The new package go/internal/auditcalibration is enrolled in go/.apicover-enforce and every exported symbol is named and covered (apicover -enforce exits 0)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run TestC1648_010_NewPackageIsEnrolledInTheRepoWideAPICoverGate ./acs/cycle1648/"
  - criterion: "The Builder's own unit tests include the negative malformed/missing-pair case and the force-override edge case, plus command-level invalid-root and deterministic-Markdown tests"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run TestC1648_011_BuilderUnitTestsCoverNegativeAndOverrideCases ./acs/cycle1648/"
---

# Eval: Auditor calibration report — mine the dossier corpus's (narrative, gate) verdict pairs

> Pins the read-only `evolve audit calibration` command (inbox
> `2026-07-30T09-02-00Z-auditor-calibration-report.json`, weight 0.86) that
> turns the verdict-conflict corpus — `knowledge-base/cycles/cycle-N.json`
> beside `.evolve/runs/cycle-N/audit-chain-shadow.json` and its optional
> `audit-fail-reason.json` — into a deterministic Markdown agreement matrix
> (auditor narrative × deterministic ship-gate outcome) with a per-defect-class
> breakdown and an explicit exclusion ledger. The gate dimension is the
> deterministic ship-gate outcome (FAIL when `overrode_by` is non-empty), not
> the reasoning-chain verdict, so cycle-1640's narrative PASS force-overridden
> to FAIL is a measured DISAGREEMENT rather than an agreement. Source incident:
> cycle 1648 — the command did not exist (`evolve: unknown command "audit"`,
> `.evolve/runs/cycle-1648/bug-reproduction-report.md`) and the corpus's
> months of (narrative, gate) pairs were unmined
> (`docs/research/llm-output-stability-2026-07/README.md`). The anti-gaming
> boundary: a report that silently drops unreadable pairs inflates agreement,
> so exclusions must be counted and listed, and invalid inputs must fail loudly
> with no partial report.

## Code Graders (bash commands that must exit 0)

- `[code]` `cd go && go test -count=1 ./internal/auditcalibration/...` — fixture corpus classifies narratives against green/red deterministic gates, keeps per-defect-class counts, and rejects malformed or missing artifact pairs as named exclusions.
- `[code]` `cd go && go test -count=1 -run 'TestAuditCalibration' ./cmd/evolve/` — the command-level tests: deterministic Markdown from a fixture corpus, non-zero for an invalid root.

## Regression Evals

- `[code]` `cd go && go test -count=1 ./internal/auditchain/... ./internal/dossier/...` — the shadow-record and dossier schemas the reader consumes stay green.

## Acceptance Checks

- `[code]` `cd go && go run ./cmd/evolve audit calibration --project-root .. --runs-dir ../.evolve/runs --dossiers-dir ../knowledge-base/cycles --output /tmp/auditor-calibration-report.md && test -s /tmp/auditor-calibration-report.md` — the real corpus produces a non-empty report whose rows distinguish narrative/gate agreement from force-overridden shipped outcomes.
- `[code]` `cd go && ! go run ./cmd/evolve audit calibration --dossiers-dir ../knowledge-base/cycles --runs-dir /definitely-missing --output /tmp/never.md` — invalid input fails loudly.

## Model-Based Checks

- `[model]` Rubric: "The generated report names its sample and exclusion rules, presents a narrative-versus-gate agreement matrix and a per-defect-class breakdown, lists force-overridden pairs separately, and does not recommend changing a persona rubric from a single anecdote." — threshold: >= 60

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cli-reachability | registered in the real dispatcher; explicit paths; documented in `evolve help` | 8/10 | `TestC1648_001_AuditCalibrationIsReachableThroughTheRealCLI` |
| distinct-dimensions | narrative / chain / gate / shipped / override columns kept apart; matrix cells exact | 8/10 | `TestC1648_002_MatrixPreservesNarrativeGateShippedAndChainDistinctly` |
| override-never-agreement | force-overridden PASS is a listed disagreement; control-corpus delta | 9/10 | `TestC1648_003_ForceOverriddenNarrativePassIsNeverCollapsedIntoAgreement` |
| loud-input-rejection | six invalid inputs → non-zero + offender named + no report | 8/10 | `TestC1648_004_InvalidCorpusInputsFailLoudly` |
| exclusions-counted-and-listed | five malformed/missing shapes are named exclusions, never matrix rows; empty corpus is zeros | 9/10 | `TestC1648_005_MalformedAndMissingPairsAreCountedAsExclusionsNeverAgreement` |
| determinism | byte-identical reruns, no timestamp, ascending order | 6/10 | `TestC1648_006_ReportIsDeterministicAcrossRuns` |
| defect-class-normalisation | reason → class before the first colon; aggregated across cycles | 7/10 | `TestC1648_007_DefectClassBreakdownCountsNormalizedReasonClasses` |
| apicover-enrollment | new package enrolled + every export named and covered | 7/10 | `TestC1648_010_NewPackageIsEnrolledInTheRepoWideAPICoverGate` |
| builder-unit-tests | negative pair case + force-override edge + command-level tests exist and pass | 7/10 | `TestC1648_011_BuilderUnitTestsCoverNegativeAndOverrideCases` |

## Thresholds

- All checks: pass@1 = 1.0
