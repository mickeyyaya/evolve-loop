# Eval: stop-criterion-secondary-phases

## Task Description
Add `## STOP CRITERION` sections to 7 secondary-phase personas with verbatim anchor phrases; add `### Scout-Report Context Diet` to triage; add `turn_budget_hint` to 6 secondary-phase profile JSONs.

## Acceptance Criteria

### AC-1: STOP CRITERION headings present in all 7 personas [code]
```bash
for f in agents/evolve-tdd-engineer.md agents/evolve-adversarial-review.md agents/evolve-architecture-design.md agents/evolve-triage.md agents/evolve-behavior-baseline.md agents/evolve-behavior-compare.md agents/evolve-retrospective.md; do
  count=$(grep -c "## STOP CRITERION" "$f" 2>/dev/null || echo 0)
  if [ "$count" -eq 0 ]; then echo "MISSING STOP CRITERION: $f"; exit 1; fi
done
echo "PASS: all 7 personas have STOP CRITERION"
```

### AC-2: Verbatim anchor phrases inside STOP CRITERION sections [code]
```bash
check_phrase() {
  local file="$1"
  local phrase="$2"
  awk '/^## STOP CRITERION/{found=1; next} found && index($0, "'"$phrase"'") > 0 {print "OK"; exit 0} found && /^## /{exit 1}' "$file" | grep -q "OK"
}
check_phrase agents/evolve-tdd-engineer.md "criteria mapping" || { echo "FAIL: criteria mapping missing in tdd-engineer STOP CRITERION"; exit 1; }
check_phrase agents/evolve-tdd-engineer.md "RED confirmation" || { echo "FAIL: RED confirmation missing in tdd-engineer STOP CRITERION"; exit 1; }
check_phrase agents/evolve-adversarial-review.md "findings classification" || { echo "FAIL: findings classification missing in adversarial-review STOP CRITERION"; exit 1; }
check_phrase agents/evolve-architecture-design.md "current-state mapping" || { echo "FAIL: current-state mapping missing in architecture-design STOP CRITERION"; exit 1; }
check_phrase agents/evolve-architecture-design.md "design decision" || { echo "FAIL: design decision missing in architecture-design STOP CRITERION"; exit 1; }
check_phrase agents/evolve-triage.md "scoped input reading" || { echo "FAIL: scoped input reading missing in triage STOP CRITERION"; exit 1; }
check_phrase agents/evolve-behavior-baseline.md "target scoping" || { echo "FAIL: target scoping missing in behavior-baseline STOP CRITERION"; exit 1; }
check_phrase agents/evolve-behavior-baseline.md "baseline report writing" || { echo "FAIL: baseline report writing missing in behavior-baseline STOP CRITERION"; exit 1; }
check_phrase agents/evolve-behavior-compare.md "input reading" || { echo "FAIL: input reading missing in behavior-compare STOP CRITERION"; exit 1; }
check_phrase agents/evolve-retrospective.md "lesson YAML verification" || { echo "FAIL: lesson YAML verification missing in retrospective STOP CRITERION"; exit 1; }
echo "PASS: all 10 anchor phrases verified inside STOP CRITERION sections"
```

### AC-3: Triage context diet section present [code]
```bash
grep -q "Scout-Report Context Diet" agents/evolve-triage.md || { echo "FAIL: context diet heading missing"; exit 1; }
echo "PASS: triage context diet section present"
```

### AC-4: Secondary profile turn_budget_hint non-null integers [code]
```bash
for f in .evolve/profiles/tdd-engineer.json .evolve/profiles/adversarial-review.json .evolve/profiles/architecture-design.json .evolve/profiles/behavior-baseline.json .evolve/profiles/behavior-compare.json .evolve/profiles/retrospective.json; do
  val=$(jq -r '.turn_budget_hint' "$f" 2>/dev/null)
  if [ -z "$val" ] || [ "$val" = "null" ]; then
    echo "FAIL: turn_budget_hint null in $f"; exit 1
  fi
  echo "OK: $f turn_budget_hint=$val"
done
echo "PASS: all 6 secondary profiles have non-null turn_budget_hint"
```

### AC-5: turn_budget_hint does not exceed max_turns [code]
```bash
check_hint_lte_max() {
  local f="$1"
  hint=$(jq -r '.turn_budget_hint' "$f")
  max=$(jq -r '.max_turns' "$f")
  if [ "$hint" -gt "$max" ] 2>/dev/null; then
    echo "FAIL: turn_budget_hint $hint exceeds max_turns $max in $f"; exit 1
  fi
  echo "OK: $f hint=$hint max=$max"
}
check_hint_lte_max .evolve/profiles/tdd-engineer.json
check_hint_lte_max .evolve/profiles/adversarial-review.json
check_hint_lte_max .evolve/profiles/architecture-design.json
check_hint_lte_max .evolve/profiles/behavior-baseline.json
check_hint_lte_max .evolve/profiles/behavior-compare.json
check_hint_lte_max .evolve/profiles/retrospective.json
echo "PASS: all turn_budget_hints within max_turns bounds"
```

### AC-6: Protected primary personas byte-identical to HEAD [code]
```bash
git diff HEAD -- agents/evolve-builder.md agents/evolve-auditor.md agents/evolve-scout.md agents/evolve-orchestrator.md agents/evolve-intent.md | wc -c | tr -d ' \n' | grep -q '^0$' || { echo "FAIL: protected persona modified"; exit 1; }
echo "PASS: primary personas unchanged"
```

### AC-7: Go suite unaffected [code]
```bash
cd go && go test ./... -count=1 -timeout 180s 2>&1 | tail -5
```

### AC-8: Negative — anchor phrase outside STOP CRITERION section does not satisfy AC-2 [model]
Verify that a hypothetical persona where the required phrase appears only before the STOP CRITERION heading would fail the awk section-scoped check in AC-2. The grader should confirm the awk logic exits 1 if the phrase appears in a prior section.
