# Eval: gap3-integrity-debugger-routing

## Goal
INTEGRITY_TREE_DRIFT (a potentially-false-positive integrity error) routes to the
debugger phase for deep-dive before a hard block, while genuinely tampered codes
(SELF_SHA_TAMPERED) still block immediately.

## Acceptance Criteria

### AC1 — INTEGRITY_TREE_DRIFT routes to debugger [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/router/... -run TestRecover_Branches -v 2>&1 | grep -q "integrity tree drift"
# The test for "integrity tree drift" must PASS (not FAIL)
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/router/... -run TestRecover_Branches -v 2>&1 | grep "integrity tree drift" | grep -v "FAIL"
```

### AC2 — SELF_SHA_TAMPERED still blocks (PhaseEnd) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/router/... -run TestRecover_Branches/integrity_self_sha -v 2>&1 | grep -q "PASS"
```

### AC3 — All router tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/router/... 2>&1 | grep -q "^ok"
```

### AC4 — GAP 3 marked DONE in self-healing-gaps.md [code]
```bash
grep -q "DONE" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
grep "GAP 3\|integrity.block\|integrity-block" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md | grep -i "DONE\|debugger.*done\|done.*debugger"
```

### AC5 — Negative: old hard-block-all-integrity behavior no longer present [code]
```bash
# The integrity-block handler must NOT unconditionally return PhaseEnd for all integrity codes
# (i.e., the code for INTEGRITY_TREE_DRIFT must route to debugger)
grep -A5 "integrity-block" /Users/danleemh/ai/claude/evolve-loop/go/internal/router/recovery.go | grep -q "debugger"
```
