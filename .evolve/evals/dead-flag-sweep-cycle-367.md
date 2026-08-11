# Eval: dead-flag-sweep-cycle-367

## Task
Remove all 18 `StatusDead` entries from `go/internal/flagregistry/registry_table.go`,
update the hand-maintained cluster tables in `docs/architecture/control-flags.md`,
regenerate the Generated Flag Index, update the cycle-354 acs tests whose positive
assertions reference these flags, and add a cycle-367 acs regression guard.

## Acceptance Criteria

### AC1 — 18 dead flags absent from flagregistry.Lookup [code]

```bash
cd go && go test -tags acs ./acs/cycle367/... -run TestC367_001
```

Expected: PASS — `flagregistry.Lookup` returns `ok=false` for all 18 removed flags.

Anti-gaming: must call the real `Lookup()` binary-search; grep on source alone is insufficient.

Negative case: if any flag is still present in registry_table.go, Lookup returns `(flag, true)` → test FAIL.

### AC2 — `evolve flags check` exits 0 (Generated Index in sync) [code]

```bash
cd go && make build && ./bin/evolve flags check
```

Expected: exit 0. Confirms the Generated Flag Index in control-flags.md matches
the updated registry (all 18 removed from the generated table too).

Anti-gaming: a stale control-flags.md with the old generated rows would exit non-zero here.

### AC3 — cycle-354 acs tests all pass [code]

```bash
cd go && go test -tags acs ./acs/cycle354/...
```

Expected: PASS. The Builder must have removed or updated the positive DEAD assertions
in amplified_test.go (Amp_001, Amp_003, Amp_004) for the 4 dead flags covered:
EVOLVE_DRY_RUN_PROVISION_WORKTREE, EVOLVE_RESOLVE_ROOTS_LOADED,
EVOLVE_FAILURE_CLASSIFICATIONS_LOADED, EVOLVE_STRICT_FAILURES.

Negative case (edge): `EVOLVE_PROFILE_WORKTREE_AWARE` remains DEPRECATED in the registry;
its Amp_001/Amp_003 assertions for `| DEAD` must be updated to reflect it is still
present (the hand-maintained table or the test itself needs updating).

### AC4 — 18 flags absent from control-flags.md entirely [code]

```bash
for f in \
  EVOLVE_ANCHOR_EXTRACT \
  EVOLVE_CARRYOVER_TODO_MAX_UNPICKED \
  EVOLVE_CONTEXT_DIGEST \
  EVOLVE_CYCLE_STATE_FILE \
  EVOLVE_DIR \
  EVOLVE_DIR_OVERRIDE \
  EVOLVE_DRY_RUN_PROVISION_WORKTREE \
  EVOLVE_FAILURE_CLASSIFICATIONS_LOADED \
  EVOLVE_FANOUT_RETROSPECTIVE \
  EVOLVE_FANOUT_SCOUT \
  EVOLVE_INSTINCT_SUMMARY_CAP \
  EVOLVE_PROFILE_OVERRIDE \
  EVOLVE_PROMPT_BUDGET_ENFORCE \
  EVOLVE_RESOLVE_ROOTS_LOADED \
  EVOLVE_STATE_FILE_OVERRIDE \
  EVOLVE_STATE_OVERRIDE \
  EVOLVE_STRICT_FAILURES \
  EVOLVE_TRIAGE_ENABLED; do
  count=$(grep -c "$f" docs/architecture/control-flags.md 2>/dev/null || echo 0)
  if [ "$count" -gt 0 ]; then echo "STILL PRESENT: $f ($count lines)"; fi
done
```

Expected: no output (all 18 absent from control-flags.md).

### AC5 — zero production readers outside acs/ [code]

```bash
for f in \
  EVOLVE_ANCHOR_EXTRACT EVOLVE_CARRYOVER_TODO_MAX_UNPICKED EVOLVE_CONTEXT_DIGEST \
  EVOLVE_CYCLE_STATE_FILE EVOLVE_DIR EVOLVE_DIR_OVERRIDE \
  EVOLVE_DRY_RUN_PROVISION_WORKTREE EVOLVE_FAILURE_CLASSIFICATIONS_LOADED \
  EVOLVE_FANOUT_RETROSPECTIVE EVOLVE_FANOUT_SCOUT EVOLVE_INSTINCT_SUMMARY_CAP \
  EVOLVE_PROFILE_OVERRIDE EVOLVE_PROMPT_BUDGET_ENFORCE EVOLVE_RESOLVE_ROOTS_LOADED \
  EVOLVE_STATE_FILE_OVERRIDE EVOLVE_STATE_OVERRIDE EVOLVE_STRICT_FAILURES \
  EVOLVE_TRIAGE_ENABLED; do
  hits=$(grep -r "$f" --include="*.go" --include="*.sh" --include="*.yml" \
    --exclude-dir=".git" --exclude-dir="acs" . 2>/dev/null | \
    grep -v "registry_table.go" | wc -l | tr -d ' ')
  if [ "$hits" -gt 0 ]; then echo "HAS PROD READER: $f ($hits hits)"; fi
done
```

Expected: no output. (Edge: acs/ tests referencing flags in string literals are OK — excluded above.)

### AC6 — active flags not over-removed [code]

```bash
cd go && make build && ./bin/evolve flags check
```

Expected: exit 0 (same command as AC2 — serves as adversarial guard that no ACTIVE/INTERNAL flag
was accidentally swept).

### AC7 — flag count reduced from 282 [code]

```bash
cd go && go test ./internal/flagregistry/... -v -run TestAll_SortedByName 2>&1 | grep PASS
```

Expected: PASS (registry still sorted). Separately verify count:

```bash
python3 -c "
import re
with open('go/internal/flagregistry/registry_table.go') as f:
    c = f.read()
n = len(re.findall(r'Status:', c))
print(f'Flag count: {n}')
assert n == 264, f'Expected 264, got {n}'
print('PASS')
"
```

Expected: `Flag count: 264` and `PASS` (282 − 18 = 264).

Negative/edge: if count is 282, no flags were removed; if count < 264, an active flag was swept.

### AC8 — cycle367 acs tests pass [code]

```bash
cd go && go test -tags acs ./acs/cycle367/...
```

Expected: PASS — all regression guards in the new `go/acs/cycle367/` package pass.
