# Eval: dead-flag-sweep-batch

## Goal
Remove all 18 StatusDead flags from `go/internal/flagregistry/registry_table.go`,
regenerate `docs/architecture/control-flags.md`, update stale hand-maintained
cluster-table rows, fix the 4 cycle354 acs FileContains assertions that check
dead flags show as "DEAD" in the doc (they'll be gone entirely), and ship a
durable regression guard in `registry_test.go` ensuring none of the 18 names
re-enter the registry.

## Acceptance Criteria

### AC1: All 18 dead flag names are absent from registry_table.go [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep -cE '"EVOLVE_ANCHOR_EXTRACT"|"EVOLVE_CARRYOVER_TODO_MAX_UNPICKED"|"EVOLVE_CONTEXT_DIGEST"|"EVOLVE_CYCLE_STATE_FILE"|"EVOLVE_DIR"|"EVOLVE_DIR_OVERRIDE"|"EVOLVE_DRY_RUN_PROVISION_WORKTREE"|"EVOLVE_FAILURE_CLASSIFICATIONS_LOADED"|"EVOLVE_FANOUT_RETROSPECTIVE"|"EVOLVE_FANOUT_SCOUT"|"EVOLVE_INSTINCT_SUMMARY_CAP"|"EVOLVE_PROFILE_OVERRIDE"|"EVOLVE_PROMPT_BUDGET_ENFORCE"|"EVOLVE_RESOLVE_ROOTS_LOADED"|"EVOLVE_STATE_FILE_OVERRIDE"|"EVOLVE_STATE_OVERRIDE"|"EVOLVE_STRICT_FAILURES"|"EVOLVE_TRIAGE_ENABLED"' \
    internal/flagregistry/registry_table.go || echo "0"
```
Expected: `0`

### AC2: Registry count drops from 282 to 264 [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep -c 'Status: Status' internal/flagregistry/registry_table.go
```
Expected: `264`

### AC3: Regression guard test passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/flagregistry/... -run TestRemovedDeadFlags_NotInRegistry_Cycle366 -v 2>&1 | tail -5
```
Expected output contains: `PASS`

### AC4: Registry unit tests all pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/flagregistry/... 2>&1 | tail -5
```
Expected: last line is `ok` with no FAIL

### AC5: evolve flags check exits 0 (generated doc matches registry, no drift) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  go run ./go/cmd/evolve flags check 2>&1 | tail -5
```
Expected: exits 0; output does NOT contain "stale" or "drift"

### AC6: CI gate (noorphan) still green [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test -tags acs ./acs/regression/noorphan/... 2>&1 | tail -5
```
Expected: last line is `ok`

### AC7: Negative — no production surface reads any removed flag [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  grep -rn 'EVOLVE_ANCHOR_EXTRACT\|EVOLVE_CARRYOVER_TODO_MAX_UNPICKED\|EVOLVE_CONTEXT_DIGEST\|EVOLVE_CYCLE_STATE_FILE\|EVOLVE_DRY_RUN_PROVISION_WORKTREE\|EVOLVE_FAILURE_CLASSIFICATIONS_LOADED\|EVOLVE_FANOUT_RETROSPECTIVE\|EVOLVE_FANOUT_SCOUT\|EVOLVE_INSTINCT_SUMMARY_CAP\|EVOLVE_PROFILE_OVERRIDE\|EVOLVE_PROMPT_BUDGET_ENFORCE\|EVOLVE_RESOLVE_ROOTS_LOADED\|EVOLVE_STATE_FILE_OVERRIDE\|EVOLVE_STATE_OVERRIDE\|EVOLVE_STRICT_FAILURES\|EVOLVE_TRIAGE_ENABLED' \
    go/ .github/ skills/ agents/ \
    --include="*.go" --include="*.yml" --include="*.yaml" --include="*.sh" \
    2>/dev/null \
  | grep -v 'registry_table.go\|control-flags.md\|_test.go\|acs/cycle354\|testdata\|acs/cycle366' \
  | wc -l | tr -d ' '
```
Expected: `0`

### AC8: Negative — EVOLVE_DIR removed too (no false-positive on exact match) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep -cE '"EVOLVE_DIR"' internal/flagregistry/registry_table.go || echo "0"
```
Expected: `0`
(Note: EVOLVE_DIR_OVERRIDE and EVOLVE_ADAPTERS_DIR_OVERRIDE are different names and must remain unaffected)

### AC9: Active integrity-gate flags unaffected (spot check) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep -c '"EVOLVE_EVAL_GATE"\|"EVOLVE_CONTRACT_GATE"\|"EVOLVE_SANDBOX"' \
    internal/flagregistry/registry_table.go
```
Expected: `3`

### AC10: Registry remains sorted by Name [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep 'Name: "EVOLVE_' internal/flagregistry/registry_table.go \
    | sed 's/.*Name: "\(EVOLVE_[^"]*\)".*/\1/' \
    | diff - <(grep 'Name: "EVOLVE_' internal/flagregistry/registry_table.go \
        | sed 's/.*Name: "\(EVOLVE_[^"]*\)".*/\1/' | sort) \
  && echo "PASS: sorted" || echo "FAIL: not sorted"
```
Expected: `PASS: sorted`

### AC11: Edge — EVOLVE_DIR_OVERRIDE (different flag) is unchanged in registry [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep -c '"EVOLVE_DIR_OVERRIDE"' internal/flagregistry/registry_table.go || echo "0"
```
Expected: `0` (it should be removed too, it is one of the 18 dead flags — if this returns 1, the builder missed it)

### AC12: Cycle354 acs tests no longer break on removed flags [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test -tags acs ./acs/cycle354/... 2>&1 | tail -5
```
Expected: last line is `ok` (no FAIL from FileContains checking removed flags in control-flags.md)
