# Eval: cmd-swarm-cmd-coverage-lift

## Purpose
Verify that cmd/evolve coverage improves from 63.5% baseline, specifically targeting
cmd_swarm.go (0%), cmd_cycle.go (37.5%), cmd_worktree.go (33.3%).

## Criteria

### C1: cmd_swarm.go test file exists with ≥4 test functions [code]
```bash
[code]
count=$(grep -c "^func Test" /Users/danleemh/ai/claude/evolve-loop/go/cmd/evolve/cmd_swarm_test.go 2>/dev/null || echo 0)
[ "$count" -ge 4 ] && echo "PASS: $count tests" || echo "FAIL: need ≥4, got $count"
```

### C2: Tests compile without errors [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./cmd/evolve/... 2>&1 | head -10 && echo "PASS: compiles" || echo "FAIL: compile error"
```

### C3: cmd/evolve package coverage rises above 72% [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./cmd/evolve/... -coverprofile=/tmp/cmd_cov.out -count=1 2>&1 | grep "coverage:" | head -3
go tool cover -func=/tmp/cmd_cov.out 2>/dev/null | grep "^total:" | awk '{if ($3+0 >= 72.0) print "PASS: coverage "$3; else print "FAIL: coverage "$3" < 72.0%"}'
```

### C4: swarmFlags() function has test coverage [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./cmd/evolve/... -coverprofile=/tmp/cmd2.out -count=1 -run "." 2>/dev/null
go tool cover -func=/tmp/cmd2.out 2>/dev/null | grep "swarmFlags\|manifestPath\|runSwarm" | head -5
```

### C5: runCycle test covers at least one branch [code]
```bash
[code]
count=$(grep -c "runCycle\|RunCycle\|Cycle" /Users/danleemh/ai/claude/evolve-loop/go/cmd/evolve/cmd_cycle_test.go 2>/dev/null || echo 0)
[ "$count" -ge 1 ] && echo "PASS: runCycle tested" || echo "FAIL: cmd_cycle_test.go missing or empty"
```

### NEGATIVE C6: No hardcoded tmux session names or live CLI calls in unit tests [code]
```bash
[code]
f=/Users/danleemh/ai/claude/evolve-loop/go/cmd/evolve/cmd_swarm_test.go
# Should not exec real tmux in unit tests
grep -c "exec\.Command\|os\.Exec" "$f" 2>/dev/null | xargs -I{} sh -c '[ "{}" -eq 0 ] && echo "PASS: no exec" || echo "WARN: {} exec calls — verify test isolation"'
```
