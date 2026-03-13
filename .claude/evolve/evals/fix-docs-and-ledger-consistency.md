# Eval: fix-docs-and-ledger-consistency

## Grader Type: grep + manual

## Checks

### 1. writing-agents.md no longer instructs "Copy the full content"
```bash
grep -n "Copy the full content" docs/writing-agents.md
```
**Expected:** Zero matches. The old copy-full-content instruction is removed.
**Verdict:** PASS if zero matches, FAIL if any match found.

### 2. writing-agents.md references context overlay / subagent_type pattern
```bash
grep -n "subagent_type\|context.overlay\|context overlay" docs/writing-agents.md
```
**Expected:** At least one match describing the overlay pattern.
**Verdict:** PASS if match found, FAIL if not.

### 3. No "timestamp" field in ledger format specifications
```bash
grep -rn '"timestamp"' agents/ skills/ docs/ --include="*.md"
```
**Expected:** Zero matches. All ledger format specs use `"ts"`.
**Verdict:** PASS if zero matches, FAIL if any match found.

### 4. Existing ledger entries use "ts" consistently
```bash
grep '"timestamp"' .claude/evolve/ledger.jsonl
```
**Expected:** Zero matches. All entries normalized to `"ts"`.
**Verdict:** PASS if zero matches, FAIL if any match found.

### 5. memory-protocol.md still specifies "ts" as canonical
```bash
grep '"ts"' skills/evolve-loop/memory-protocol.md
```
**Expected:** At least one match confirming `"ts"` is the canonical field.
**Verdict:** PASS if match found, FAIL if not.

## Overall Verdict
PASS if all 5 checks pass. FAIL if any check fails.
