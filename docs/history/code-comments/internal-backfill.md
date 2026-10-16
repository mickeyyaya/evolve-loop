# Comment history: `internal/backfill`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d12

### `go/internal/backfill/backfill_test.go:3` — above `import (`

```text
// RED tests for cycle-171 T3 (artifact-backfill). The backfill package does not
// exist yet, so this test file fails to BUILD at the baseline (undefined:
// TryExtract) — the correct RED for a brand-new package, isolated to this
// package's own test binary. Builder makes it GREEN by creating backfill.go with
//
//	func TryExtract(workspace, phase, artifactPath string, minLen int) (bool, error)
//
// which reads <workspace>/<phase>-stdout.clean.txt, finds the LAST occurrence of
// the phase's markdown header (scout→"# Scout Report", build→"# Build Report",
// audit→"# Audit Report", tdd→"# TDD", intent→"# Intent", triage→"# Triage"),
// extracts header..EOF (trimmed), and — only when len >= minLen — writes it to
// artifactPath atomically. Returns (true,nil) on success, (false,nil) on
// no-header / too-short / unknown-phase, (false,err) on I/O error.
```

### `go/internal/backfill/backfill_test.go:158` — above `func TestTryExtract_Retro_PositiveWritesArtifact(t *testing.T) {`

```text
// --- cycle-187: backfill coverage extended to retro + build-planner ---
```

### `go/internal/backfill/backfill_test.go:160` — above `func TestTryExtract_Retro_PositiveWritesArtifact(t *testing.T) {`

```text
// AC-1 / AC-7 (cycle-187): "retro" is a newly backfill-covered phase. The
// retrospective agent's stdout carries the canonical "# Retrospective Report"
// header — the exact form every real retrospective-report.md on disk uses
// (e.g. ".evolve/runs/cycle-29/retrospective-report.md" → "# Retrospective
// Report — Cycle 29"). With "retro" in phaseHeaders, that report must be
// reconstructed and written to the artifact path.
//
// RED at baseline: phaseHeaders has no "retro" entry → TryExtract takes the
// unknown-phase branch and returns (false, nil) → ok==false, artifact absent.
```

### `go/internal/backfill/backfill_test.go:193` — above `func TestTryExtract_BuildPlanner_PositiveWritesArtifact(t *testing.T) {`

```text
// AC-2 / AC-7 (cycle-187): "build-planner" is a newly backfill-covered phase.
// Its artifact is build-plan.md, headed "# Build Plan" (real on-disk form:
// "# Build Plan — Cycle 121"). With "build-planner" in phaseHeaders, the plan
// must be reconstructed from its stdout.clean.txt.
//
// RED at baseline: no "build-planner" entry in phaseHeaders → (false, nil).
```
