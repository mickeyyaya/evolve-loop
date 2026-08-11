# Eval: persona-tools-coherence-gate

## Task
Implement a Go linter that compares the `tools:` frontmatter array in `agents/evolve-*.md` files to the `allowed_tools` in the corresponding `.evolve/profiles/*.json`. Report contradictions; surface as ACS predicate.

## Criteria

### C1: evolve phases check-coherence exits 0 on current set [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && go/evolve phases check-coherence 2>&1
echo "exit=$?"
```
Expected: exit 0 (or exit 2 only for known, documented contradictions filed as issues)

### C2: Go unit test catches persona declaring a disallowed tool [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -count=1 -run TestCoherence_PersonaDeclares_Disallowed 2>&1
```
Expected: exit 0, PASS — test verifies that a persona listing "Write" when the profile has no Write in allowed_tools is flagged

### C3: Go unit test catches profile allowing a tool the persona doesn't declare [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -count=1 -run TestCoherence_ProfileAllows_UndeclaredTool 2>&1
```
Expected: exit 0, PASS — the test verifies that a profile allowing WebSearch while the persona tools: list omits it is flagged as a WARN

### C4 (negative): Mismatch detection works end-to-end via compiled binary [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -count=1 -run TestCoherence_MismatchDetected 2>&1
```
Expected: exit 0, PASS — unit test confirms contradiction detection without requiring env-var injection

### C5: ACS predicate exists and passes via compiled binary [code]
```bash
test -f /Users/danleemh/ai/claude/evolve-loop/acs/cycle-239/001-persona-tools-coherence.sh && \
  bash /Users/danleemh/ai/claude/evolve-loop/acs/cycle-239/001-persona-tools-coherence.sh 2>&1
```
Expected: exit 0

### C6: Content-parity assertion — phasecoherence package present in worktree [code]
```bash
git -C /Users/danleemh/ai/claude/evolve-loop diff HEAD -- go/internal/phasecoherence/ | grep -q "^++" && echo "PASS: phasecoherence changes present" || { echo "FAIL: no phasecoherence diff"; exit 1; }
```
Expected: exit 0, "PASS: phasecoherence changes present"

### C7 (regression): Full test suite green [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... -count=1 2>&1 | grep "^FAIL" | head -5
```
Expected: no output (no failures)
