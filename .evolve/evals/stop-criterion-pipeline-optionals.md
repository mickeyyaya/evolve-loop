# Eval: stop-criterion-pipeline-optionals

## Task Description
Add `## STOP CRITERION` sections to 3 pipeline-optional phase personas: `agents/evolve-test-amplification.md`, `agents/evolve-tester.md`, and `agents/evolve-changelog-sync.md`. Each section must include at least 2 named completion gates and a hard turn-exit trigger appropriate to the phase.

## Acceptance Criteria

### AC-1: STOP CRITERION headings in all 3 pipeline-optional personas [code]
```bash
for f in agents/evolve-test-amplification.md agents/evolve-tester.md agents/evolve-changelog-sync.md; do
  count=$(grep -c "## STOP CRITERION" "$f" 2>/dev/null || echo 0)
  if [ "$count" -eq 0 ]; then
    echo "MISSING STOP CRITERION: $f"; exit 1
  fi
done
echo "PASS: all 3 pipeline-optional personas have STOP CRITERION"
```

### AC-2: Each STOP CRITERION section contains at least 2 gate names [code]
```bash
count_gates() {
  local file="$1"
  awk '/^## STOP CRITERION/{found=1; next} found && /^## /{exit} found && /gate|complete|written|verified|done/{n++} END{print n+0}' "$file"
}
for f in agents/evolve-test-amplification.md agents/evolve-tester.md agents/evolve-changelog-sync.md; do
  gates=$(count_gates "$f")
  if [ "$gates" -lt 2 ]; then
    echo "FAIL: fewer than 2 gates in STOP CRITERION of $f (found $gates)"; exit 1
  fi
  echo "OK: $f has $gates gate references"
done
echo "PASS: all STOP CRITERION sections have ≥2 gates"
```

### AC-3: Hard turn-exit trigger present in each section [code]
```bash
check_turn_exit() {
  local file="$1"
  awk '/^## STOP CRITERION/{found=1; next} found && /^## /{exit} found && /turn [0-9]/{print "OK"; exit}' "$file" | grep -q "OK" || { echo "FAIL: no turn-exit trigger in $file STOP CRITERION"; exit 1; }
}
check_turn_exit agents/evolve-test-amplification.md
check_turn_exit agents/evolve-tester.md
check_turn_exit agents/evolve-changelog-sync.md
echo "PASS: all STOP CRITERION sections have turn-exit triggers"
```

### AC-4: Protected primary personas unchanged [code]
```bash
git diff HEAD -- agents/evolve-builder.md agents/evolve-auditor.md agents/evolve-scout.md agents/evolve-orchestrator.md agents/evolve-intent.md agents/evolve-triage.md agents/evolve-tdd-engineer.md agents/evolve-adversarial-review.md agents/evolve-architecture-design.md agents/evolve-behavior-baseline.md agents/evolve-behavior-compare.md agents/evolve-retrospective.md | wc -c | tr -d ' \n' | grep -q '^0$' || { echo "FAIL: non-target persona modified"; exit 1; }
echo "PASS: non-target personas unchanged"
```

### AC-5: Negative — empty STOP CRITERION header with no gates fails intent [model]
Confirm that a STOP CRITERION section consisting only of the heading and no gate bullets would fail AC-2 (gate count = 0 < 2). The model grader should verify the awk gate-count logic correctly returns 0 for an empty section body.

### AC-6: Go suite unaffected [code]
```bash
cd go && go test ./... -count=1 -timeout 180s 2>&1 | tail -5
```
