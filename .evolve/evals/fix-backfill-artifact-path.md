# Eval: fix-backfill-artifact-path

## Summary
Verifies that the backfill artifact path is correct for all phases (especially tdd and intent),
that backfill is enabled by default in CLAUDE.md, and that the docs reflect the correct paths.

---

## Criterion 1 — orchestrator uses correct artifact path for tdd phase [code]

```bash
grep -n "backfillArtifactPath\|test-report.md" \
  go/internal/core/orchestrator.go | grep -v "^$"
```

Expected: at least one hit referencing `test-report.md` or a `backfillArtifactPath` helper that maps `tdd` to `test-report.md`.

---

## Criterion 2 — orchestrator uses correct artifact path for intent phase [code]

```bash
grep -n "intent\.md\|intent-delta\|backfillArtifactPath" \
  go/internal/core/orchestrator.go | grep -v "^$"
```

Expected: at least one hit showing `intent.md` is used for intent phase backfill (not `intent-report.md`).

---

## Criterion 3 — tdd artifact path helper or switch maps correctly [code]

```bash
go test ./go/internal/core/... -run TestOrchestrator_Backfill -v 2>&1 | tail -20
```

Expected: all backfill tests pass (exit 0).

---

## Criterion 4 — CLAUDE.md shows EVOLVE_BACKFILL_ENABLED default-on [code]

```bash
grep "EVOLVE_BACKFILL_ENABLED" CLAUDE.md | head -5
```

Expected: default shown as `1` (not `0`).

---

## Criterion 5 — docs artifact-backfill.md shows correct paths for tdd and intent [code]

```bash
grep -E "tdd|intent" docs/architecture/artifact-backfill.md | head -10
```

Expected: `tdd` row shows `test-report.md` (not `tdd-report.md`) and `intent` row shows `intent.md` (not `intent-report.md`).

---

## Negative: wrong path constant must NOT exist in orchestrator after fix [code]

```bash
grep -n '"tdd-report\.md"\|"intent-report\.md"' go/internal/core/orchestrator.go
```

Expected: no output (exit non-zero or empty — these wrong paths must not remain).

---

## Edge: triage phase still maps to triage-report.md [code]

```bash
grep -n "triage" go/internal/core/orchestrator.go | grep -i "backfill\|artifactPath\|report" | head -5
```

Expected: `triage-report.md` is either produced by default `<phase>-report.md` logic or an explicit correct mapping.
