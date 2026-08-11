# Eval: Coverage gate — flip warning-only to enforcement

> Cycle 193 LOW task. Changes `.github/workflows/go.yml` coverage gate from
> `exit 0` (warning-only; fail=1 set but ignored) to `exit $fail` so packages
> below 85% actually fail CI. The "Phase 1 build: warning-only" placeholder
> comment is removed or updated.

## AC-1: exit 0 bypass replaced in coverage gate [code]
```bash
# The "exit 0" that ignores $fail must be gone from the coverage gate step
# We check that within the coverage gate step, only "exit $fail" or similar enforcement exists
grep -A 25 "coverage gate" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/go.yml | \
  grep -v "::warning::" | grep "exit 0" && { echo "FAIL: exit 0 bypass still present"; exit 1; } || \
  echo "PASS: no bare exit 0 in coverage gate"
```

## AC-2: exit $fail (enforcement) present in coverage gate [code]
```bash
grep -A 30 "coverage gate" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/go.yml | \
  grep -qE "exit \\\$fail|exit_code=\\\$fail" && echo "PASS" || { echo "FAIL: exit \$fail not found"; exit 1; }
```

## AC-3 (negative): warning annotation still present for visibility [code]
```bash
grep -q "::warning::coverage" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/go.yml && \
  echo "PASS: warning annotation retained" || { echo "FAIL: warning annotation removed"; exit 1; }
```

## AC-4: CI YAML is valid (no syntax break) [code]
```bash
python3 -c "import yaml, sys; yaml.safe_load(open('/Users/danleemh/ai/claude/evolve-loop/.github/workflows/go.yml'))" && \
  echo "PASS: valid YAML" || { echo "FAIL: YAML parse error"; exit 1; }
```

## AC-5 (negative): warning-only comment removed [code]
```bash
# The "Phase 1 build: warning-only" placeholder should not remain
grep -q "Phase 1 build.*warning.only\|warning-only until task" \
  /Users/danleemh/ai/claude/evolve-loop/.github/workflows/go.yml && \
  { echo "FAIL: stale 'warning-only' comment still present"; exit 1; } || \
  echo "PASS: stale warning-only comment removed"
```
