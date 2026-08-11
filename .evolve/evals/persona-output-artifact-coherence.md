# Eval: persona-output-artifact-coherence

## Task
Implement a check that the `output-format:` frontmatter field in `agents/evolve-*.md` references the same artifact basename as `output_artifact` in the corresponding profile. Gate: mismatches exit non-zero.

## Criteria

### C1: evolve phases check-artifact-coherence exits 0 on current set [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && go/evolve phases check-artifact-coherence 2>&1
echo "exit=$?"
```
Expected: exit 0 — all currently-paired personas have matching artifact names between their output-format: frontmatter and profile output_artifact

### C2: Go unit test detects output-format/output_artifact mismatch [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -count=1 -run TestArtifactCoherence_Mismatch 2>&1
```
Expected: exit 0, PASS — test proves that a persona with `output-format: "old-name.md"` while profile declares `output_artifact: ".../new-name.md"` is flagged

### C3 (negative — edge case): Persona with no output-format frontmatter is not false-positived [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -count=1 -run TestArtifactCoherence_NoFrontmatter 2>&1
```
Expected: exit 0, PASS — personas without output-format: line (non-output phases) are silently skipped, not flagged

### C4 (negative — profile without output_artifact): Profile has no output_artifact, persona has output-format — flagged as WARN [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -count=1 -run TestArtifactCoherence_ProfileMissingField 2>&1
```
Expected: exit 0, PASS

### C5: ACS predicate exists and passes via compiled binary [code]
```bash
test -f /Users/danleemh/ai/claude/evolve-loop/acs/cycle-239/002-persona-artifact-coherence.sh && \
  bash /Users/danleemh/ai/claude/evolve-loop/acs/cycle-239/002-persona-artifact-coherence.sh 2>&1
```
Expected: exit 0

### C6 (regression): Go tests green [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... -count=1 2>&1 | grep "^FAIL" | head -5
```
Expected: no output (no failures)
