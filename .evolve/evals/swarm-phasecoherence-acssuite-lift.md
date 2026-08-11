# Eval: swarm-phasecoherence-acssuite-lift

## Purpose
Verify coverage improvements in swarm, phasecoherence, and acssuite packages —
targeting functions at 0-57% coverage.

## Criteria

### C1: phasecoherence package coverage ≥ 90% [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -coverprofile=/tmp/pc.out -count=1 2>&1 | grep "coverage:"
go tool cover -func=/tmp/pc.out 2>/dev/null | grep "^total:" | awk '{if ($3+0 >= 90.0) print "PASS: "$3; else print "FAIL: "$3" < 90%"}'
```

### C2: canonicalRole function has ≥80% coverage [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -coverprofile=/tmp/pc2.out -count=1 2>/dev/null
go tool cover -func=/tmp/pc2.out 2>/dev/null | grep "canonicalRole" | awk '{if ($3+0 >= 80.0) print "PASS: canonicalRole "$3; else print "FAIL: canonicalRole "$3" < 80%"}'
```

### C3: swarm package coverage ≥ 90% [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/swarm/... -coverprofile=/tmp/sw.out -count=1 2>&1 | grep "coverage:"
go tool cover -func=/tmp/sw.out 2>/dev/null | grep "^total:" | awk '{if ($3+0 >= 90.0) print "PASS: "$3; else print "FAIL: "$3" < 90%"}'
```

### C4: acssuite package coverage ≥ 88% [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/acssuite/... -coverprofile=/tmp/acs.out -count=1 2>&1 | grep "coverage:"
go tool cover -func=/tmp/acs.out 2>/dev/null | grep "^total:" | awk '{if ($3+0 >= 88.0) print "PASS: "$3; else print "FAIL: "$3" < 88%"}'
```

### C5: All new tests pass [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... ./internal/swarm/... ./internal/acssuite/... ./internal/phases/swarmrunner/... ./internal/phases/swarmplan/... -count=1 2>&1 | grep -E "^(ok|FAIL)" | head -20
```

### NEGATIVE C6: canonicalRole rejects unknown roles — not a no-op [code]
```bash
[code]
# Verify test exists for unknown/invalid role input to canonicalRole
grep -qi "unknown\|invalid\|empty\|unrecognized" /Users/danleemh/ai/claude/evolve-loop/go/internal/phasecoherence/provenance_test.go 2>/dev/null && echo "PASS: edge case tested" || echo "FAIL: no negative/edge test for canonicalRole"
```
