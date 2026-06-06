---
score_cap:
  - criterion: "All 17 TestResearchPhasesAreConfigOnly cases pass (3 Product wave cases added: opportunity-map, prd-draft, metric-tree)"
    max_if_missing: 5
    evidence: "cd go && go test ./internal/phasespec/ -run 'TestResearchPhasesAreConfigOnly$' -count=1"
  - criterion: "All 12 Product config files present on disk and git-tracked"
    max_if_missing: 6
    evidence: "bash acs/cycle-10/004-product-config-files-present.sh"
  - criterion: "agents/ mirrors byte-identical to phase-dir agent.md for all 3 Product phases"
    max_if_missing: 7
    evidence: "bash acs/cycle-10/005-product-mirrors-byte-identical.sh"
  - criterion: "evolve phases validate exits 0 (OK) for opportunity-map, prd-draft, metric-tree"
    max_if_missing: 6
    evidence: "bash acs/cycle-10/006-validate-opportunity-map.sh && bash acs/cycle-10/007-validate-prd-draft.sh && bash acs/cycle-10/008-validate-metric-tree.sh"
  - criterion: "Archetypes match spec §3 Wave Product table (plan/plan/evaluate)"
    max_if_missing: 6
    evidence: "bash acs/cycle-10/013-product-archetype-correctness.sh"
  - criterion: "Catalog wave-status table: Product flipped to done, Integration remains queued"
    max_if_missing: 6
    evidence: "bash acs/cycle-10/011-catalog-product-status-flip.sh && bash acs/cycle-10/012-catalog-integration-still-queued.sh"
---

# Eval: wave-product-discovery-tdd-and-phases

> Pins Wave Product (product-discovery domain, domain-phase-catalog.md §3):
> `opportunity-map` (plan, Torres Opportunity Solution Tree), `prd-draft`
> (plan, SVPG/Lenny PRD convergence), `metric-tree` (evaluate, Amplitude
> North Star Framework — the only Product phase emitting a verdict
> vocabulary per ADR-0035). All three are zero-Go user phases delivered as
> pure config (12 files) + 3 test cases in TestResearchPhasesAreConfigOnly.
> Source incident: cycle-9 audit FAIL — `013-product-archetype-correctness.sh`
> was grep-only WITHOUT the `acs-predicate: config-check` waiver (Level 0,
> blocking_count=1) and C8 incorrectly required Ops to stay queued (Ops
> shipped in cycle 5). Both defects are corrected in this revision (cycle 10).
> Authored per cycle-131 lesson (missing `.evolve/evals/<slug>.md` =
> automatic CRITICAL FAIL at audit).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| catalog-contract | 17/17 catalog cases pass incl. 3 Product | 5/10 | `go test -run 'TestResearchPhasesAreConfigOnly$'` |
| config-presence | 12 Product config files tracked | 6/10 | `acs/cycle-10/004` |
| mirror-discipline | agents/ mirrors byte-identical | 7/10 | `acs/cycle-10/005` |
| validator-floor | phases validate OK for all 3 | 6/10 | `acs/cycle-10/006-008` |
| archetype-fidelity | plan/plan/evaluate per spec §3 | 6/10 | `acs/cycle-10/013` |
| wave-status | Product done, Integration queued | 6/10 | `acs/cycle-10/011-012` |

## Objective
Verify that Wave Product (product-discovery domain) is delivered as pure config: 3 test cases added to TestResearchPhasesAreConfigOnly, 12 config files authored (4 per phase), all phases validate OK, and no production Go was modified.

**Cycle-10 note:** Retry of cycle-9 task that failed audit. Two fixes applied: (1) C7 carries the `acs-predicate: config-check` waiver (behavioral consequence of archetype covered by C1/C6 Go contract tests); (2) C8 corrects downstream-wave check — Ops is ✅ done (cycle 5), only Integration remains ⬜ queued.

## Criteria

### C1 — Test suite green: all 3 Product wave cases pass [code]
```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE/go" || exit 1
output=$(go test ./internal/phasespec/... -run TestResearchPhasesAreConfigOnly -count=1 -v 2>&1)
echo "$output"
for phase in opportunity-map prd-draft metric-tree; do
  echo "$output" | grep -q "PASS: TestResearchPhasesAreConfigOnly/$phase" || {
    echo "FAIL: test case $phase not PASS"
    exit 1
  }
done
echo "PASS: all 3 Product wave test cases green"
```

### C2 — All 12 config files exist (phase.json + agent.md + agents mirror + profile) [code]
```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || exit 1
FAIL=0
for phase in opportunity-map prd-draft metric-tree; do
  for f in \
    ".evolve/phases/$phase/phase.json" \
    ".evolve/phases/$phase/agent.md" \
    "agents/evolve-$phase.md" \
    ".evolve/profiles/$phase.json"; do
    test -f "$f" || { echo "FAIL: missing $f"; FAIL=1; }
  done
done
[ "$FAIL" -eq 0 ] && echo "PASS: all 12 config files present" || exit 1
```

### C3 — agent.md and agents mirror are byte-identical [code]
```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || exit 1
FAIL=0
for phase in opportunity-map prd-draft metric-tree; do
  diff ".evolve/phases/$phase/agent.md" "agents/evolve-$phase.md" > /dev/null 2>&1 || {
    echo "FAIL: .evolve/phases/$phase/agent.md and agents/evolve-$phase.md differ"
    FAIL=1
  }
done
[ "$FAIL" -eq 0 ] && echo "PASS: all agent.md files are byte-identical to their agents/ mirrors" || exit 1
```

### C4 — evolve phases validate exits 0 for all 3 phases [code]
```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || exit 1
for phase in opportunity-map prd-draft metric-tree; do
  EVOLVE_PROJECT_ROOT="$(pwd)" go run ./go/cmd/evolve phases validate "$phase" 2>&1 | grep -q "^OK" || {
    echo "FAIL: $phase validate failed"
    exit 1
  }
done
echo "PASS: all 3 phases validate OK"
```

### C5 — No non-config Go files modified (scope guard) [code]
```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || exit 1
CHANGED=$(git diff HEAD~1 --name-only 2>/dev/null | grep "\.go$" | grep -v "usercatalog_research_test.go" || true)
if [ -n "$CHANGED" ]; then
  echo "FAIL: unexpected Go file changes (scope creep): $CHANGED"
  exit 1
fi
echo "PASS: only permitted Go file changed (usercatalog_research_test.go)"
```

### C6 — metric-tree (evaluate) has verdict vocabulary via phasecontract derivation [code]
```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE/go" || exit 1
# Behavioral: the Go test asserts c.Verdicts non-empty for metric-tree (evaluate archetype + verdict_on_pass).
output=$(go test ./internal/phasespec/... -run 'TestResearchPhasesAreConfigOnly/metric-tree$' -count=1 -v 2>&1)
echo "$output"
echo "$output" | grep -q "PASS:" || { echo "FAIL: metric-tree (evaluate) contract test failed"; exit 1; }
echo "PASS: metric-tree verdict vocabulary confirmed via contract test"
```

### C7 — Archetype correctness: opportunity-map=plan, prd-draft=plan, metric-tree=evaluate [code]
```bash
#!/usr/bin/env bash
# acs-predicate: config-check — archetype is a declarative phase.json field;
# the behavioral consequence (verdict vocabulary presence/absence) is covered
# by C1 and C6 via the Go contract tests. This predicate pins the declared
# field per spec §3 Wave Product table. Grep waiver per tdd-engineer
# predicate-quality classification (Auditor reviews validity).
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || exit 1
fail=0
check_archetype() {
  local phase="$1" want="$2"
  local f=".evolve/phases/$phase/phase.json"
  if [ ! -f "$f" ]; then
    echo "RED: $f missing" >&2; fail=1; return
  fi
  if ! grep -q "\"archetype\": \"$want\"" "$f"; then
    echo "RED: $phase archetype != $want" >&2; fail=1
  fi
}
check_archetype opportunity-map plan
check_archetype prd-draft plan
check_archetype metric-tree evaluate
# Negative: plan phases must NOT carry evaluate archetype.
for phase in opportunity-map prd-draft; do
  if [ -f ".evolve/phases/$phase/phase.json" ] \
    && grep -q '"archetype": "evaluate"' ".evolve/phases/$phase/phase.json"; then
    echo "RED: $phase incorrectly declared evaluate (spec §3 says plan)" >&2; fail=1
  fi
done
[ "$fail" -eq 0 ] || exit 1
echo "GREEN: archetypes match spec §3 (plan/plan/evaluate)"
```

### C8 — Product status row flipped; Integration wave still queued [code]
```bash
#!/usr/bin/env bash
# acs-predicate: config-check — wave-status table rows are inherent doc-presence
# checks. Grep waiver per tdd-engineer predicate-quality classification.
# NOTE: Ops is done (cycle 5) — only check Product (flip) and Integration (stays open).
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || exit 1
CATALOG="docs/architecture/domain-phase-catalog.md"
[ -f "$CATALOG" ] || { echo "FAIL: $CATALOG missing"; exit 1; }
# Positive: Product row must be done.
prod_row=$(grep -E '^\| *Product *\|' "$CATALOG" | head -1)
if [ -z "$prod_row" ]; then
  echo "FAIL: Product row missing from wave-status table"; exit 1
fi
if ! echo "$prod_row" | grep -q '✅'; then
  echo "FAIL: Product row not marked done: $prod_row"; exit 1
fi
if echo "$prod_row" | grep -q '⬜'; then
  echo "FAIL: Product row still queued: $prod_row"; exit 1
fi
# Negative: Integration must still be queued (ships next cycle).
int_row=$(grep -E '^\| *Integration *\|' "$CATALOG" | head -1)
if [ -z "$int_row" ]; then
  echo "FAIL: Integration row missing from wave-status table"; exit 1
fi
if ! echo "$int_row" | grep -q '⬜'; then
  echo "FAIL: Integration row should remain queued: $int_row"; exit 1
fi
echo "PASS: Product flipped to done; Integration remains queued"
```
