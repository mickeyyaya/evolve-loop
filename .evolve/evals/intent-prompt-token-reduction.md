# Eval: intent-prompt-token-reduction

**Task slug:** `intent-prompt-token-reduction`
**Target file:** `agents/evolve-intent.md`
**Intent:** Remove redundant / verbose / historical-archaeology instructional text from the Intent-phase agent prompt while preserving ALL behavioral instructions and required sections. Baseline (pre-change): 234 lines, 12769 bytes.

The reduction must be *realized in the committed file* (fewer lines AND fewer bytes), not scaffolded. Behavior-bearing instructions (schema, AwN classifier, >=1 challenged-premise rule, STOP criterion, turn-budget numbers, delta-mode contract, reflection step) MUST survive in meaning.

## Acceptance Criteria

### 1. The file shrank -- strict line reduction `[code]`
```bash
test "$(wc -l < agents/evolve-intent.md)" -lt 234
```
Expected: exit 0 (line count strictly below the 234-line baseline).

### 2. The reduction is meaningful, not cosmetic -- >=15 lines removed `[code]`
```bash
test "$(wc -l < agents/evolve-intent.md)" -le 219
```
Expected: exit 0. Guards against a one-line "reduction" that games criterion 1. (Cheapest fake = delete a single blank line -> fails here.)

### 3. Byte count fell too `[code]`
```bash
test "$(wc -c < agents/evolve-intent.md)" -lt 12769
```
Expected: exit 0 (token proxy: fewer bytes than the 12769-byte baseline).

### 4. NEGATIVE CASE -- dead historical archaeology is gone `[code]`
```bash
! grep -qE "C69|cycle 11 measured|No web research deadline|v9\.0\.1 design correction" agents/evolve-intent.md
```
Expected: exit 0 (none of the removable historical-justification markers remain). This is the *purpose* of the change; if this content still exists, the reduction did not happen. (Cheapest fake = delete unrelated blank lines while keeping the archaeology -> fails here.)

### 5. Behavior preserved -- every required instructional anchor still present `[code]`
```bash
for s in \
  "name: evolve-intent" \
  "awn_class" \
  "Ask-when-Needed" \
  "IMKI" "IMR" "IwE" "IBTC" "CLEAR" \
  "challenged_premise" \
  "at least one" \
  "STOP CRITERION" \
  "Maximum 2 turns" \
  "gate_intent_to_research" \
  "INTENT_MODE" \
  "intent-unchanged" \
  "acceptance_checks" \
  "Reflection Authoring"; do
  grep -qF "$s" agents/evolve-intent.md || { echo "MISSING ANCHOR: $s"; exit 1; }
done
echo "ALL_ANCHORS_PRESENT"
```
Expected: exit 0, prints `ALL_ANCHORS_PRESENT`. Any deleted behavioral instruction fails the eval -- this defeats the "shrink by deleting required content" gaming path.

### 6. EDGE CASE -- frontmatter still valid `[code]`
```bash
head -1 agents/evolve-intent.md | grep -qx -- "---" && grep -qE "^model: " agents/evolve-intent.md
```
Expected: exit 0 (file still opens with a `---` YAML frontmatter fence and retains the `model:` key -- structural integrity intact after edits).

### 7. Diff stays scoped to agent markdown `[code]`
```bash
git diff --name-only HEAD | grep -vqE '^agents/evolve-intent\.md$' && echo "OUT_OF_SCOPE" && exit 1; exit 0
```
Expected: exit 0 (no non-`agents/evolve-intent.md` files in the working diff; enforces the goal's 1-3-file / agent-markdown-only constraint and the no-control-plane rule).

## Grading
All seven checks are `[code]` graders and must pass. Criteria 4 and 7 are negative/scope cases; criterion 6 is an edge case (structural integrity). Together the only way to pass = genuinely remove redundant prose while preserving behavior and scope. Command-verb diversity: `test`/`wc`/`grep -qE`/`grep -qF`/`head`/`git diff` -- no diversity collapse.
