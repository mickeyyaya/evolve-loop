# Eval: backfill-phase-coverage

## Goal

Verify that the artifact backfill system correctly covers the `retro` and `build-planner`
phases, that the retro phase runner polls for the file the agent actually writes
(`retrospective-report.md`), and that the stale "11-signal" comment in cyclehealth.go
is updated to "13-signal".

---

## Acceptance Criteria

### AC-1: backfill phaseHeaders covers retro
`phaseHeaders["retro"]` in `go/internal/backfill/backfill.go` is set to `"# Retrospective Report"`
(or a prefix that matches the retro agent's actual output header).

```bash [code]
grep -q '"retro":' go/internal/backfill/backfill.go
```

### AC-2: backfill phaseHeaders covers build-planner
`phaseHeaders["build-planner"]` in `go/internal/backfill/backfill.go` is set to `"# Build Plan"`
(or a prefix that matches the build-planner agent's actual output header).

```bash [code]
grep -q '"build-planner":' go/internal/backfill/backfill.go
```

### AC-3: backfillArtifactPath returns correct path for retro
`backfillArtifactPath(ws, "retro")` returns a path ending in `retrospective-report.md`
(not the stale default `retro-report.md`).

```bash [code]
grep -q '"retro"' go/internal/core/orchestrator.go && grep -A3 '"retro"' go/internal/core/orchestrator.go | grep -q 'retrospective-report'
```

### AC-4: backfillArtifactPath returns correct path for build-planner
`backfillArtifactPath(ws, "build-planner")` returns a path ending in `build-plan.md`
(not the stale default `build-planner-report.md`).

```bash [code]
grep -q '"build-planner"' go/internal/core/orchestrator.go && grep -A3 '"build-planner"' go/internal/core/orchestrator.go | grep -q 'build-plan'
```

### AC-5: retro.go polls for retrospective-report.md
`retro.go` sets its `artifactPath` to `retrospective-report.md` (aligning with what the
retrospective agent actually writes), not the stale `retrospective.md`.

```bash [code]
grep -q 'retrospective-report.md' go/internal/phases/retro/retro.go
```

### AC-6: retro_test.go updated
`retro_test.go` ArtifactPath assertion expects `retrospective-report.md`.

```bash [code]
grep -q 'retrospective-report.md' go/internal/phases/retro/retro_test.go
```

### AC-7: New backfill tests pass for retro and build-planner
Table-driven or distinct tests exist in `backfill_test.go` for phase "retro" and
"build-planner" extraction.

```bash [code]
grep -q '"retro"' go/internal/backfill/backfill_test.go && grep -q '"build-planner"' go/internal/backfill/backfill_test.go
```

### AC-8: All backfill tests pass
```bash [code]
cd go && go test ./internal/backfill/ -count=1 -timeout 30s
```

### AC-9: All retro tests pass
```bash [code]
cd go && go test ./internal/phases/retro/... -count=1 -timeout 30s
```

### AC-10: cyclehealth.go comment updated to 13 signals
The package-level comment no longer says "11-signal".

```bash [code]
! grep -q '11-signal' go/internal/cyclehealth/cyclehealth.go
```

### AC-11: All cyclehealth tests pass
```bash [code]
cd go && go test ./internal/cyclehealth/ -count=1 -timeout 30s
```

---

## Negative / Edge Cases

### NEG-1: Unknown phase still returns false (no regression)
`TryExtract` with an unknown phase like "bogus" still returns `(false, nil)`.

```bash [code]
cd go && go test ./internal/backfill/ -run TestTryExtract_UnknownPhaseReturnsFalse -v -timeout 30s
```

### NEG-2: backfillArtifactPath for known phases not broken
The TDD path still maps to "test-report.md" and "intent" still maps to "intent.md".

```bash [code]
cd go && go test ./internal/core/ -run TestOrchestrator_Backfill_TDDArtifactPath -v -timeout 120s
```
