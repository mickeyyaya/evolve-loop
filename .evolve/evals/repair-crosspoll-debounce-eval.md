---
score_cap:
  - criterion: "the permanent eval entry .evolve/evals/artifact-ready-crosspoll-debounce.md exists, is well-formed, and its evidence commands all exit 0"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs ./acs/cycle1258/... -run TestC1258_005_PermanentEvalEntryExistsAndItsEvidenceRuns -v"
  - criterion: "the eval file is force-tracked past .gitignore so ship's dropIgnoredPaths cannot silently drop it again"
    max_if_missing: 6
    evidence: "git ls-files --error-unmatch .evolve/evals/artifact-ready-crosspoll-debounce.md"
---

# Eval: Repair the cycle-1258 audit prescription (crosspoll-debounce eval)

> Pins the concrete instance F3 names (batch-integrity-review-2026-08-04.md):
> cycle-1258's auditor WARN-shipped with a structured prescription —
> materialize `.evolve/evals/artifact-ready-crosspoll-debounce.md`, `git add -f`
> past `.gitignore` — and the prescription was never applied across the
> 1233→1249→1252→1254→1258 salvage chain. `TestC1258_005_PermanentEvalEntryExistsAndItsEvidenceRuns`
> (`go/acs/cycle1258/predicates_test.go:318`) is the regression lock, live-RED
> as of cycle 1375's scout: `predicates_test.go:323: permanent eval entry
> .evolve/evals/artifact-ready-crosspoll-debounce.md is absent`. This entry caps
> future audits at a reduced score until the Builder materializes the eval file
> and force-tracks it past `.gitignore`, closing the instance defect. Source
> incident: cycle-1198 (the original debounce defect); prescription drop:
> cycles 1233–1258 (audit's own WARN never enforced downstream).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| unrepaired prescription | permanent eval entry exists + evidence runs | 7/10 | `go test -tags acs ./acs/cycle1258/... -run TestC1258_005` |
| gitignore silent-drop | eval file is force-tracked, not re-dropped by ship | 6/10 | `git ls-files --error-unmatch .evolve/evals/artifact-ready-crosspoll-debounce.md` |
