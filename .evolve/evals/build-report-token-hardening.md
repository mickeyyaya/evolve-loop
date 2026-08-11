# Eval: build-report-token-hardening

## Task Summary
Harden the builder agent reference so the build-report challenge token is always sourced
from workspace/challenge-token.txt (the canonical source), never copied from prior-cycle
artifacts. This fixes the H1 failure class from cycle-223.

## Acceptance Criteria

### AC1 — Builder reference contains challenge-token.txt read instruction [code]
```bash
cd "$(git rev-parse --show-toplevel)"
F="agents/evolve-builder-reference.md"
grep -q "challenge-token.txt" "$F" || { echo "RED: $F has no mention of challenge-token.txt" >&2; exit 1; }
echo "GREEN: $F references challenge-token.txt"
```

### AC2 — Builder reference warns against copying from prior artifacts [code]
```bash
cd "$(git rev-parse --show-toplevel)"
F="agents/evolve-builder-reference.md"
# Must contain language that prohibits copying token from prior-cycle artifacts
grep -qiE "never.*(copy|use).*prior|prior.*(cycle|artifact|report).*token|do not copy.*token" "$F" \
  || { echo "RED: $F missing 'never copy token from prior artifact' warning" >&2; exit 1; }
echo "GREEN: $F contains prior-artifact anti-copy warning for challenge token"
```

### AC3 — ACS predicate for token-hardening exists and is behavioral [code]
```bash
cd "$(git rev-parse --show-toplevel)"
P=$(ls acs/cycle-224/*token*.sh 2>/dev/null | head -1)
[ -n "$P" ] || { echo "RED: no token-hardening ACS predicate found in acs/cycle-224/" >&2; exit 1; }
grep -qE "challenge-token\.txt|challenge_token" "$P" || {
  echo "RED: predicate $P does not reference challenge-token.txt" >&2; exit 1
}
echo "GREEN: token-hardening ACS predicate exists: $P"
```

### AC4 (negative) — Template placeholder not raw in builder reference [code]
```bash
cd "$(git rev-parse --show-toplevel)"
F="agents/evolve-builder-reference.md"
# The template shows {challengeToken} as a placeholder — that's fine.
# But the instruction section must clarify it comes from workspace, not copy-paste.
# Ensure the existing line 318-ish template comment survived (we are patching, not removing).
grep -q "challenge-token:" "$F" || { echo "RED: challenge-token template line removed from $F" >&2; exit 1; }
echo "GREEN: challenge-token template line preserved in $F"
```

### AC5 — ACS suite cycle-224 still green after both task changes [code]
```bash
cd "$(git rev-parse --show-toplevel)"
OUT=$(./go/bin/evolve acs suite --cycle 224 2>&1)
echo "$OUT" | grep -q "verdict=PASS" || { echo "RED: ACS suite broke after token-hardening change: $OUT" >&2; exit 1; }
echo "GREEN: ACS suite remains green: $OUT"
```
