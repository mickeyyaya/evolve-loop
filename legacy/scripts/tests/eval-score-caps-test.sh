#!/bin/bash
# eval-score-caps-test.sh — Verify score cap enforcement in eval-quality-check.sh
#
# Tests: cap fires on synthetic eval, score_caps_ceiling correct in JSON output.

set -uo pipefail

PASS=0
FAIL=0
SCRIPT="scripts/verification/eval-quality-check.sh"

check() {
  local desc="$1" result="$2"
  if [ "$result" = "0" ]; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc"
    FAIL=$((FAIL + 1))
  fi
}

# T1-T3: synthetic eval with a cap whose evidence always fails (exit 1)
TF=$(mktemp /tmp/eval-score-cap-test-XXXXXX.md)
cat > "$TF" <<'EVALEOF'
---
score_cap:
  criterion: "always absent"
  max_if_missing: 6
  evidence: "exit 1"
---

# Eval: synthetic

## Code Graders

- `grep -c "score_cap" scripts/verification/eval-quality-check.sh | awk '{exit ($1 < 1)}'`
EVALEOF

OUTPUT=$(bash "$SCRIPT" "$TF" 2>/dev/null || true)
rm -f "$TF"

# T1: score_caps_ceiling is 6 (the max_if_missing value)
printf '%s' "$OUTPUT" | grep -qE '"score_caps_ceiling":[[:space:]]*6'
check "score_caps_ceiling:6 when cap fires (max_if_missing=6)" "$?"

# T2: caps_fired annotation appears in Issues output (stderr is suppressed; check via ceiling != null)
printf '%s' "$OUTPUT" | grep -qE '"score_caps_ceiling":[[:space:]]*[0-9]'
check "score_caps_ceiling is numeric (not null) when cap fires" "$?"

# T3: overall JSON structure is valid (has expected keys)
printf '%s' "$OUTPUT" | grep -q '"totalCommands"'
check "JSON output contains totalCommands key" "$?"

# T4-T5: cap does NOT fire when evidence exits 0
TF2=$(mktemp /tmp/eval-score-cap-test-XXXXXX.md)
cat > "$TF2" <<'EVALEOF'
---
score_cap:
  criterion: "always present"
  max_if_missing: 5
  evidence: "true"
---

# Eval: synthetic passing

## Code Graders

- `grep -c "score_cap" scripts/verification/eval-quality-check.sh | awk '{exit ($1 < 1)}'`
EVALEOF

OUTPUT2=$(bash "$SCRIPT" "$TF2" 2>/dev/null || true)
rm -f "$TF2"

# T4: score_caps_ceiling is null when evidence passes (cap does not fire)
printf '%s' "$OUTPUT2" | grep -qE '"score_caps_ceiling":[[:space:]]*null'
check "score_caps_ceiling:null when evidence exits 0 (no cap fires)" "$?"

# T5: file with no frontmatter returns null ceiling
TF3=$(mktemp /tmp/eval-score-cap-test-XXXXXX.md)
printf '# Eval: no frontmatter\n\n## Code Graders\n\n- `grep -q "x" scripts/verification/eval-quality-check.sh`\n' > "$TF3"
OUTPUT3=$(bash "$SCRIPT" "$TF3" 2>/dev/null || true)
rm -f "$TF3"

printf '%s' "$OUTPUT3" | grep -qE '"score_caps_ceiling":[[:space:]]*null'
check "score_caps_ceiling:null for eval with no score_cap frontmatter" "$?"

# Summary
echo ""
echo "${PASS}/$(( PASS + FAIL )) PASS"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
