# Eval: research-missing-dev-phases-kb

## Objective
Verify that a comprehensive KB research article on missing development phases was created with substantive content covering multiple phases with citations and implementation guidance.

## Criteria

### C1 — File exists with minimum structure [code]
```bash
#!/bin/bash
set -euo pipefail
FILE="knowledge-base/research/missing-development-phases-2026-06.md"
test -f "$FILE" || { echo "FAIL: $FILE not found"; exit 1; }
SECTIONS=$(grep -c "^## " "$FILE" || true)
[ "$SECTIONS" -ge 5 ] || { echo "FAIL: only $SECTIONS top-level sections, expected >= 5"; exit 1; }
echo "PASS: $FILE exists with $SECTIONS sections"
```

### C2 — Contains at least 3 distinct phase descriptions [code]
```bash
#!/bin/bash
FILE="knowledge-base/research/missing-development-phases-2026-06.md"
# Each phase should have a heading and description
PHASES=$(grep -c "^### " "$FILE" || true)
[ "$PHASES" -ge 3 ] || { echo "FAIL: only $PHASES subsections, expected >= 3 phase entries"; exit 1; }
echo "PASS: $PHASES phase entries found"
```

### C3 — Includes pipeline placement guidance [code]
```bash
#!/bin/bash
FILE="knowledge-base/research/missing-development-phases-2026-06.md"
grep -qi "pipeline\|phase.*after\|phase.*before\|fits.*between\|insert.*after" "$FILE" || {
  echo "FAIL: no pipeline placement guidance found in $FILE"; exit 1;
}
echo "PASS: pipeline placement guidance present"
```

### C4 — Contains external source citations [code]
```bash
#!/bin/bash
FILE="knowledge-base/research/missing-development-phases-2026-06.md"
# Should have links or source references
CITATIONS=$(grep -c "https://\|arXiv\|arxiv\|github.com\|\- Source\|\- URL\|| URL " "$FILE" || true)
[ "$CITATIONS" -ge 3 ] || { echo "FAIL: only $CITATIONS citations, expected >= 3"; exit 1; }
echo "PASS: $CITATIONS citations found"
```

### C5 — Does not replace existing phases (additive only) [code]
```bash
#!/bin/bash
FILE="knowledge-base/research/missing-development-phases-2026-06.md"
# Should not claim to remove any existing phases
! grep -qi "remove.*phase\|delete.*phase\|replace.*scout\|replace.*audit\|replace.*build" "$FILE" || {
  echo "FAIL: document suggests removing existing phases"; exit 1;
}
echo "PASS: additive-only, no phase removal language"
```
