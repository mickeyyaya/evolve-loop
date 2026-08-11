# Eval: prompt-ondemand-section-strip

## Goal
Strip `## Reference Index` and other on-demand-only sections from agent prompt bodies when `EVOLVE_COMPACT_PROMPTS=1`, reducing prompt token count for incremental cycles.

## Acceptance Criteria

### 1. Strip function compiles and passes unit tests [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/prompts/... -run TestStripOnDemandSections -v 2>&1
# Expected: PASS — StripOnDemandSections("body\n## Reference Index\ntable") returns "body"
```

### 2. Prompt byte count reduced when flag enabled [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/runner/... -run TestCompactPromptShrinks -v 2>&1
# Expected: PASS — compact prompt is strictly smaller than full prompt for agent bodies with Reference Index sections
```

### 3. Env flag EVOLVE_COMPACT_PROMPTS=0 leaves prompt unchanged [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/runner/... -run TestCompactPrompt_Disabled -v 2>&1
# Expected: PASS — with flag off, body returned verbatim byte-for-byte
```

### 4. Build clean with no regressions [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./... 2>&1 | head -20
# Expected: zero build errors
```

### 5. Negative: section header not present → body returned unchanged [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/prompts/... -run TestStripOnDemandSections_NoSection -v 2>&1
# Expected: PASS — body with no Reference Index section is returned verbatim
```

### 6. Existing agent load tests still pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/prompts/... -v 2>&1 | tail -10
# Expected: all existing tests PASS, no regression
```
