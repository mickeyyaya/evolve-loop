# Eval: backfill-fix-and-enable

**Slug:** backfill-fix-and-enable  
**Phase:** build  
**Cycle:** 177

## Objective

Verify that the backfill artifact path bug is fixed (tdd→test-report.md, intent→intent.md),
that `backfill.ArtifactFilename()` is exported, that `EVOLVE_BACKFILL_ENABLED` is documented
as default-on, and that documentation is updated.

---

## Criteria

### C1 — Correct artifact filename exported from backfill package [code]

```bash
grep -n "test-report.md" /Users/danleemh/ai/claude/evolve-loop/go/internal/backfill/backfill.go
```

Expected: at least one line containing `test-report.md` (mapping for tdd phase).

### C2 — intent.md mapped correctly in backfill package [code]

```bash
grep -n '"intent".*"intent\.md"\|"intent\.md".*"intent"' /Users/danleemh/ai/claude/evolve-loop/go/internal/backfill/backfill.go
```

Expected: a line mapping the intent phase to `intent.md`.

### C3 — orchestrator no longer hardcodes generic -report.md [code]

```bash
grep -n 'string(next)+"-report.md"' /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
```

Expected: empty output — the hardcoded generic path must be replaced.

### C4 — orchestrator uses backfill.ArtifactFilename [code]

```bash
grep -n "backfill.ArtifactFilename\|ArtifactFilename" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
```

Expected: at least one line containing `backfill.ArtifactFilename` in orchestrator.go.

### C5 — backfill.go compiles and tests pass [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/backfill/... 2>&1
```

Expected: `ok` line — no FAIL.

### C6 — core package tests pass [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... 2>&1 | grep -E "^ok|^FAIL"
```

Expected: `ok` line — no FAIL.

### C7 — CLAUDE.md documents EVOLVE_BACKFILL_ENABLED as default-on [code]

```bash
grep -A2 "EVOLVE_BACKFILL_ENABLED" /Users/danleemh/ai/claude/evolve-loop/CLAUDE.md | head -10
```

Expected: line showing `1` as the default (e.g., `\| 1 (default-on)\|` or similar).

### C8 — artifact-backfill.md mentions classify-bypass limitation [code]

```bash
grep -i "classify\|bypass\|quality" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/artifact-backfill.md | head -5
```

Expected: at least one matching line noting that classify is skipped on backfill.

### C9 — negative: tdd still maps to wrong path (regression guard) [code]

```bash
grep -n '"tdd".*"tdd-report\.md"\|"tdd-report\.md"' /Users/danleemh/ai/claude/evolve-loop/go/internal/backfill/backfill.go
```

Expected: empty output — `tdd-report.md` must NOT appear in the backfill package.

### C10 — negative: intent-report.md must not appear in backfill [code]

```bash
grep -n '"intent-report\.md"' /Users/danleemh/ai/claude/evolve-loop/go/internal/backfill/backfill.go
```

Expected: empty output — `intent-report.md` must NOT appear.
