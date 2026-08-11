# Eval: artifact-backfill

<!-- challenge-token: bd89ec70f56cebf4 -->

## Task
Implement a backfill package `go/internal/backfill/` that extracts a phase's markdown artifact
from `<phase>-stdout.clean.txt` when the artifact file was never written. Wire it into the
orchestrator's `ErrArtifactTimeout` exhaustion path under `EVOLVE_BACKFILL_ENABLED=1`.

## Acceptance Criteria

### AC-1: backfill package exists [code]
```bash
ls /Users/danleemh/ai/claude/evolve-loop/go/internal/backfill/backfill.go
# Should exit 0 — package file must exist
```

### AC-2: TryExtract extracts content from clean.txt [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/backfill/... -v 2>&1 | grep -E "PASS|FAIL|RUN"
# Should find tests and all pass
```

### AC-3: TryExtract returns false for unknown phase [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/backfill/... -run "TestTryExtract.*Unknown\|TestBackfill.*NoHeader\|TestBackfill.*Unknown" -v 2>&1 | grep -E "PASS|FAIL|RUN"
# Should pass — unknown phases / missing headers must return false
```

### AC-4: TryExtract returns false when content below min length [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/backfill/... -run "TestTryExtract.*Short\|TestBackfill.*TooShort\|TestBackfill.*Short" -v 2>&1 | grep -E "PASS|FAIL|RUN"
# Should pass — too-short content must not be backfilled
```

### AC-5: Orchestrator wiring references EVOLVE_BACKFILL_ENABLED [code]
```bash
grep -q "EVOLVE_BACKFILL_ENABLED\|BackfillEnabled\|backfill.*enabled" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
# Should exit 0 — env gate must be present in orchestrator
```

### AC-6: Negative — EVOLVE_BACKFILL_ENABLED=0 skips backfill (default behavior unchanged) [code]
```bash
# Build must not regress
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... ./internal/backfill/... 2>&1 | tail -5
# Should exit 0 — no regressions in core or backfill
```

### AC-7: CLAUDE.md env-var table updated [code]
```bash
grep -q "EVOLVE_BACKFILL_ENABLED" /Users/danleemh/ai/claude/evolve-loop/CLAUDE.md
# Should exit 0 — env var documented in CLAUDE.md
```
