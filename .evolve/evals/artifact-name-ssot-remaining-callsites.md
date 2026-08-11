---
score_cap:
  - criterion: "No production Go file under go/internal declares a phase-report filename (scout/build/tdd/audit) as its own string literal — only the phasecontract registry does"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1152_003_no_report_filename_literals_outside_phasecontract ./acs/cycle1152"
  - criterion: "The phasecontract registry remains the runtime-truth SSOT for artifact names, including the phases whose registry name diverges from the <phase>-report.md convention (tdd, retro, build-planner) and the NoArtifact/unregistered fallbacks"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1152_001_registry_is_the_artifact_name_ssot ./acs/cycle1152"
  - criterion: "The tdd ArtifactFilename hook and ship's manifestReportFiles resolve their filenames through phasecontract rather than carrying a literal copy"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1152_002_remaining_callsites_resolve_through_ssot ./acs/cycle1152"
  - criterion: "No migrated call site substitutes the hand-rolled `phase + \"-report.md\"` convention for the registry lookup, and the six sites migrated in cycle-1149 do not regress"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1152_004_no_handrolled_convention_and_no_regression ./acs/cycle1152"
  - criterion: "The whole module still compiles — routing the remaining call sites through phasecontract introduced no import cycle"
    max_if_missing: 9
    evidence: "cd go && go build ./..."
---

# Eval: Artifact-name SSOT — remaining `*-report.md` call sites

> Pins the artifact-filename SSOT invariant established by
> `phasecontract.ArtifactName`/`ArtifactFilename`
> (`go/internal/phasecontract/contract_registry.go:241-275`). Cycle-1145 added the
> SSOT and backfilled the `scout-report.md` sites; cycle-1149 migrated six more
> files (`phases/build`, `phases/audit`, `core/phase_bindings`,
> `core/build_removal_check`, `coherence`, `consensusdispatch`); cycle-1152
> closes the last two (`phases/tdd/tdd.go:38`, `phases/ship/manifest.go:53`).
> That is exactly the shape of the cycle-1145 retro-phase mismatch: the registry
> moved, the literals did not. This eval keeps the substitution from silently
> regrowing — a future consumer that re-types a report filename caps the audit
> score at 8/10 — while the behavioral caps ensure the de-duplication never
> breaks the readers that consume those artifacts.
>
> **Corrected premise (cycle-1152).** The cycle-1149 revision of this eval
> carried a `scope-boundary` cap asserting that ship's `manifestReportFiles`
> must RETAIN `test-report.md` as a literal, on the ground that "'test' has no
> registry phase". That rested on a wrong registry key: the phase is `tdd`, and
> `contract_registry.go:132` registers it with `ArtifactName: "test-report.md"`.
> `ArtifactName("test")` returns `""` because `"test"` is not a phase at all —
> not because the name lacks an SSOT. The cap has been replaced by the
> anti-gaming/no-regression cap above, which bounds the same risk (a sweep that
> shrinks ship's manifest coverage) without pinning the defect.
>
> Source incidents: cycle-1145 (SSOT introduced), cycle-1149 (partial
> migration + the wrong-key premise), cycle-1152 (completion + correction).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| literal-regrowth | Report filenames declared only in the registry, checked by AST (string-literal nodes), so prose mentions in comments and error messages are correctly ignored | 8/10 | `go test -tags acs -run TestC1152_003_...` |
| ssot-anti-gaming | Registry returns runtime truth for the DIVERGENT phases (tdd→test-report.md) plus NoArtifact and unregistered fallbacks | 7/10 | `go test -tags acs -run TestC1152_001_...` |
| callsite-migration | tdd hook + ship manifest resolve through phasecontract | 6/10 | `go test -tags acs -run TestC1152_002_...` |
| wrong-fix-rejection | No hand-rolled `phase + "-report.md"` (which yields `tdd-report.md` — the exit-81 timeout tdd.go documents), and cycle-1149's six migrated files stay migrated | 7/10 | `go test -tags acs -run TestC1152_004_...` |
| import-cycle | Whole module compiles after the migration | 9/10 | `cd go && go build ./...` |
