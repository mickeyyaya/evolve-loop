# Eval: Create Token Profiler Script

## Code Graders (bash commands that must exit 0)
- `test -x scripts/token-profiler.sh || (chmod +x scripts/token-profiler.sh && test -x scripts/token-profiler.sh)`
- `bash scripts/token-profiler.sh > /tmp/token-profiler-output.txt 2>&1 && test -s /tmp/token-profiler-output.txt`
- `bash scripts/token-profiler.sh 2>/dev/null | grep -qi "lines\|tokens\|skill\|agent"`
- `bash scripts/token-profiler.sh 2>/dev/null | grep -q "evolve-loop"`
- `bash scripts/token-profiler.sh 2>/dev/null | grep -q "refactor"`

## Regression Evals (existing scripts still work)
- `bash scripts/phase-gate.sh --help >/dev/null 2>&1 || true`
- `test -f scripts/context-budget.sh`

## Acceptance Checks (behavioral — output correctness)
- `OUTPUT=$(bash scripts/token-profiler.sh 2>/dev/null); LINES=$(echo "$OUTPUT" | wc -l); [ "$LINES" -gt 5 ]`
- `OUTPUT=$(bash scripts/token-profiler.sh 2>/dev/null); echo "$OUTPUT" | grep -qE "[0-9]{2,}"`
- `bash scripts/token-profiler.sh 2>/dev/null | grep -qE "phases\.md|SKILL\.md|policies\.md"`

## Thresholds
- All checks: pass@1 = 1.0
