# Eval: remove-sandbox-dead-flags

## Goal
Verify that the two Sandbox Cluster dead flags (EVOLVE_FORCE_INNER_SANDBOX,
EVOLVE_INNER_SANDBOX) have been removed from the registry, the generated doc,
and that a regression guard prevents re-introduction.

## Acceptance Criteria

### AC1: Registry rows are absent [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep -c '"EVOLVE_FORCE_INNER_SANDBOX"\|"EVOLVE_INNER_SANDBOX"' \
    internal/flagregistry/registry_table.go || echo "0"
```
Expected: `0`

### AC2: Lookup returns ok=false for both removed flags [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./acs/cycle360/... -run TestRemovedSandboxClusterFlags -v 2>&1 | tail -5
```
Expected output contains: `PASS`

### AC3: control-flags.md no longer lists them as active or deprecated [code]
```bash
grep -c "EVOLVE_INNER_SANDBOX\|EVOLVE_FORCE_INNER_SANDBOX" \
  /Users/danleemh/ai/claude/evolve-loop/docs/architecture/control-flags.md || echo "0"
```
Expected: `0`

### AC4: evolve flags check passes (no doc drift) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  go run ./go/cmd/evolve flags check 2>&1 | tail -3
```
Expected: exits 0; output does NOT contain "stale"

### AC5: Negative — EVOLVE_SANDBOX (the active replacement) is still present [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep -c '"EVOLVE_SANDBOX"' internal/flagregistry/registry_table.go
```
Expected: `1`

### AC6: Negative — no production Go file reads the removed flags [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep -rn '"EVOLVE_FORCE_INNER_SANDBOX"\|"EVOLVE_INNER_SANDBOX"' \
    --include="*.go" . | grep -v "registry_table\|_test.go\|cycle360" \
    | wc -l | tr -d ' '
```
Expected: `0`

### AC7: Edge — registry remains sorted (Name field collation order preserved) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep 'Name: "EVOLVE_' internal/flagregistry/registry_table.go \
    | sed 's/.*Name: "\(EVOLVE_[^"]*\)".*/\1/' \
    | diff - <(grep 'Name: "EVOLVE_' internal/flagregistry/registry_table.go \
        | sed 's/.*Name: "\(EVOLVE_[^"]*\)".*/\1/' | sort) \
    && echo "PASS: sorted" || echo "FAIL: not sorted"
```
Expected: `PASS: sorted`

### AC8: Regression guard test file exists [code]
```bash
test -f /Users/danleemh/ai/claude/evolve-loop/go/acs/cycle360/predicates_test.go \
  && echo "PASS" || echo "FAIL"
```
Expected: `PASS`
