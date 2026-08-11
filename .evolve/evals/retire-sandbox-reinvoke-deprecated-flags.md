# Eval: retire-sandbox-reinvoke-deprecated-flags

## Task
Remove 4 deprecated flags whose only readers were in adapters/claude.sh (deleted in Wave A5):
EVOLVE_FORCE_INNER_SANDBOX, EVOLVE_INNER_SANDBOX, EVOLVE_PROFILE_WORKTREE_AWARE, EVOLVE_REINVOKE_CMD

## Acceptance Criteria

### AC1 — 4 flags absent from registry [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
for f in EVOLVE_FORCE_INNER_SANDBOX EVOLVE_INNER_SANDBOX EVOLVE_PROFILE_WORKTREE_AWARE EVOLVE_REINVOKE_CMD; do
  if grep -q "\"$f\"" go/internal/flagregistry/registry_table.go; then
    echo "FAIL: $f still in registry_table.go"; exit 1
  fi
done
echo "PASS: all 4 flags removed from registry"
```
Expected: exits 0, prints "PASS: all 4 flags removed from registry"

### AC2 — flags index in sync after removal [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && go/bin/evolve flags check
```
Expected: exits 0, prints "flags: index in sync"

### AC3 — flags absent from generated index [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
for f in EVOLVE_FORCE_INNER_SANDBOX EVOLVE_INNER_SANDBOX EVOLVE_PROFILE_WORKTREE_AWARE EVOLVE_REINVOKE_CMD; do
  if grep -q "$f" docs/architecture/control-flags.md; then
    echo "FAIL: $f still appears in control-flags.md"; exit 1
  fi
done
echo "PASS: all 4 flags absent from control-flags.md"
```
Expected: exits 0 (flags gone from doc)

### AC4 — flagregistry unit tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && go test -C go ./internal/flagregistry/... -count=1
```
Expected: exits 0

### AC5 — ACS regression guard passes (no new orphans, no broken ACS) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  go test -C go -tags acs ./acs/regression/flagreaders/... && \
  go test -C go -tags acs ./acs/cycle354/... && \
  go test -C go -tags acs ./acs/cycle365/...
```
Expected: all pass (exits 0)

### AC5-neg — removed flags have no production reader (anti-gaming) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
for f in EVOLVE_FORCE_INNER_SANDBOX EVOLVE_INNER_SANDBOX EVOLVE_PROFILE_WORKTREE_AWARE EVOLVE_REINVOKE_CMD; do
  hits=$(grep -r "$f" go/ .github/ skills/ agents/ \
    --include="*.go" --include="*.yml" --include="*.yaml" --include="*.md" --include="*.sh" \
    2>/dev/null \
    | grep -v "registry_table\|control-flags\|testdata\|_test.go\|flag-reduction-campaign\|plans/" \
    | wc -l | tr -d ' ')
  if [ "$hits" -gt 0 ]; then
    echo "FAIL: $f still has $hits production reference(s)"; exit 1
  fi
done
echo "PASS: zero production readers for all 4 flags"
```
Expected: exits 0

### AC6 — registry count reduced by exactly 4 [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
count=$(grep -c 'Name:' go/internal/flagregistry/registry_table.go)
if [ "$count" -ne 278 ]; then
  echo "FAIL: expected 278 flags, got $count"; exit 1
fi
echo "PASS: registry has 278 flags (was 282)"
```
Expected: exits 0, prints "PASS: registry has 278 flags (was 282)"
