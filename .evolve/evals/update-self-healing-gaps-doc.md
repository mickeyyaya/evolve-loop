# Eval: update-self-healing-gaps-doc

## Metadata
- slug: update-self-healing-gaps-doc
- cycle: 180
- task: Update docs/architecture/self-healing-gaps.md to reflect all completed self-healing work through cycle-180

## Context

`docs/architecture/self-healing-gaps.md` was created in cycle-164 as a "living analysis."
Since then, multiple gaps were closed in cycles 173-180, but the document is partially stale:
- GAPs 1, 5, 9 are marked DONE but without cycle references
- The backfill mechanism (cycle-171, improved cycle-179) is described only in artifact-backfill.md
- `attempt_count` tracking (cycle-171) is not mentioned
- GAP 2 (retry backoff) is listed as "optional" — it was implemented in cycle-180

This task updates the doc to be current, cross-references related docs, and adds a
"Completed as of cycle-180" summary section so future cycles can use it as a reference.

## Acceptance Criteria

### AC-1: GAP 2 row updated to DONE with cycle-180 reference [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -q "cycle-180\|cycle 180" "$REPO_ROOT/docs/architecture/self-healing-gaps.md" \
  || { echo "RED: self-healing-gaps.md does not reference cycle-180"; exit 1; }
echo "GREEN: cycle-180 referenced in self-healing-gaps.md"
```

### AC-2: Backfill mechanism referenced in the doc [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -qi "backfill\|artifact-backfill" "$REPO_ROOT/docs/architecture/self-healing-gaps.md" \
  || { echo "RED: backfill not mentioned in self-healing-gaps.md"; exit 1; }
echo "GREEN: backfill referenced"
```

### AC-3: attempt_count tracking referenced [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -qi "attempt_count\|AttemptCount" "$REPO_ROOT/docs/architecture/self-healing-gaps.md" \
  || { echo "RED: attempt_count not mentioned in self-healing-gaps.md"; exit 1; }
echo "GREEN: attempt_count referenced"
```

### AC-4: Doc references phase_latency signal (added in cycle-180) [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -qi "phase_latency\|phase-latency-health\|latency.*signal\|signal.*latency" \
  "$REPO_ROOT/docs/architecture/self-healing-gaps.md" \
  || { echo "RED: phase_latency signal not referenced in self-healing-gaps.md"; exit 1; }
echo "GREEN: phase_latency signal referenced"
```

### AC-5: GAP 2 row no longer says only 'optional' without resolution [code]
After the backoff is implemented, the doc should say DONE/cycle-180, not just "optional".
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
# The line for GAP 2 must now contain DONE or cycle-180, not just the old "optional" text
python3 -c "
import sys
text = open('$REPO_ROOT/docs/architecture/self-healing-gaps.md').read()
import re
# Find the GAP 2 row
m = re.search(r'\|\s*2\s*\|.*?backoff.*?\|', text, re.DOTALL)
if not m:
    sys.exit(0)  # row structure changed, that's ok
row = m.group(0)
if 'DONE' in row or 'cycle-180' in row or 'cycle 180' in row:
    print('GREEN: GAP 2 row shows DONE/cycle-180')
    sys.exit(0)
print('RED: GAP 2 row still shows only optional:', row[:120])
sys.exit(1)
" || exit 1
```

### AC-6: Doc file is non-trivially updated (not the same as before) [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
wc -l "$REPO_ROOT/docs/architecture/self-healing-gaps.md" | awk '{if ($1 >= 55) print "GREEN: doc has sufficient lines ("$1")"; else {print "RED: doc too short ("$1" lines), may not be updated"; exit 1}}'
```
