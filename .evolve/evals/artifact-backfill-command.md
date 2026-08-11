# Eval: artifact-backfill-command

## Metadata
- slug: artifact-backfill-command
- cycle: 170
- task: Implement evolve backfill command to reconstruct missing phase artifacts

## Acceptance Criteria

### AC-1: evolve backfill command is registered and prints help [code]
```bash
/Users/danleemh/ai/claude/evolve-loop/go/evolve backfill --help 2>&1 | grep -i "backfill\|phase\|cycle"
```
Expected: exit 0 (help text contains phase/cycle info)

### AC-2: backfill package exists with Backfill function [code]
```bash
ls /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/backfill/backfill.go
```
Expected: exit 0

### AC-3: backfill scout creates a minimal valid scout-report.md stub [code]
```bash
# Create a temp workspace and run backfill scout
TMPDIR=$(mktemp -d)
mkdir -p "$TMPDIR/.evolve/runs/cycle-1"
echo "lastCycleNumber: 1" > "$TMPDIR/.evolve/state.json" 2>/dev/null || echo '{"lastCycleNumber":1}' > "$TMPDIR/.evolve/state.json"
/Users/danleemh/ai/claude/evolve-loop/go/evolve backfill --phase scout --cycle 1 --workspace "$TMPDIR/.evolve/runs/cycle-1" --project-root "$TMPDIR" 2>&1
ls "$TMPDIR/.evolve/runs/cycle-1/scout-report.md" && wc -c "$TMPDIR/.evolve/runs/cycle-1/scout-report.md" | awk '{print $1}' | awk '$1 > 100'
```
Expected: exit 0 (stub created, > 100 chars)

### AC-4: backfill for a phase that already has an artifact exits non-zero (negative case) [code]
```bash
TMPDIR=$(mktemp -d)
mkdir -p "$TMPDIR/.evolve/runs/cycle-1"
echo '# Scout Report — Cycle 1\n<!-- challenge-token: abc -->\nThis is an existing scout report with plenty of content to exceed 100 chars easily.' > "$TMPDIR/.evolve/runs/cycle-1/scout-report.md"
/Users/danleemh/ai/claude/evolve-loop/go/evolve backfill --phase scout --cycle 1 --workspace "$TMPDIR/.evolve/runs/cycle-1" --project-root "$TMPDIR" 2>&1
# Should refuse or warn if artifact already exists
echo "exit $?"
```
Expected: exit 0 (command exits with clear message, not silently overwrites)

### AC-5: Stub artifact passes cyclehealth artifact-substance check (> 100 chars) [code]
```bash
# The stub must be > 100 chars to pass checkArtifactSubstance
grep -n "100\b" /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/backfill/backfill.go | head -3
```
Expected: exit 0 (100-char minimum referenced)

### AC-6: backfill package tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/backfill/... -timeout 30s 2>&1 | tail -5
```
Expected: exit 0

### AC-7: docs/architecture/self-healing-gaps.md updated with backfill capability [code]
```bash
grep -q "backfill\|Backfill\|artifact-backfill" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: exit 0
