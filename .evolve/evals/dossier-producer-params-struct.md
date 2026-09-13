---
score_cap:
  - criterion: "writeCycleDossier takes a cycleDossierParams value (≤ 3 parameters: lock, params, optional context) carrying ProjectRoot, WorkspacePath, Cycle, Goal, RunID, Outcome, SkippedPhases, VerdictsNotAdopted, SpineFailOpens and PhaseTimings by name, and a caller naming only the fields it has compiles and fabricates no evidence"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestWriteCycleDossier_ParamsAreKeyedAndOptional$' ./internal/core"
  - criterion: "For the fixed golden input the refactored producer writes byte-identical cycle-4242.json and cycle-4242.md to the pre-refactor eleven-argument producer (testdata/dossierparams, captured at 287aa81c)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes$' ./internal/core"
  - criterion: "The normal and abnormal closeout producers still emit valid dossiers (lock serialization, FAIL defects, not-adopted records, clean tree) through the params value"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestWriteCycleDossier_|TestDossierVerdict_|TestDossier_|TestDossierFailure_)' ./internal/core"
---

# Eval: writeCycleDossier takes one named params value, not eleven positional arguments

> Pins the techdebt item `2026-09-11T05-00-00Z-dossier-producer-params-struct.json`
> (weight 0.45, raised by the go-reviewer on the dossier-evidence PR as MEDIUM,
> non-blocking). `core.writeCycleDossier` grew to eleven positional parameters
> by accretion; adding the eleventh (live phase timings, the cycle-1623
> dossier-evidence fix) touched every call site purely to append a value. One
> frame down the same code already shows the better shape —
> `dossier.Build(cycle, dossier.BuildOpts{...})`. The producer now takes a
> `cycleDossierParams` value mirroring `BuildOpts`; a field addition touches
> only the producer and the site that supplies it. Behaviour is pinned as
> byte-equivalence against a golden captured through the OLD signature, so the
> refactor cannot drop, rename, or re-map an input. RED authored in cycle 1652.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| params-value | ≤ 3 params, ten named fields, keyed construction, no fabricated evidence | 5/10 | `go test -run TestWriteCycleDossier_ParamsAreKeyedAndOptional ./internal/core` |
| byte-equivalence | golden JSON + Markdown unchanged for the fixed input | 4/10 | `go test -run TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes ./internal/core` |
| closeout-unchanged | pre-existing producer/lock/failure/not-adopted tests green through the new signature | 6/10 | `go test -run 'TestWriteCycleDossier_\|TestDossierVerdict_\|TestDossier_\|TestDossierFailure_' ./internal/core` |
