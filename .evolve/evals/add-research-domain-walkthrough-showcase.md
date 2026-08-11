# Eval: add-research-domain-walkthrough-showcase

## Code Graders (bash commands that must exit 0)

1. Research Domain Walkthrough section present:
   ```bash
   grep -q "## Research Domain Walkthrough" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md
   ```

2. Groundedness check eval pattern demonstrated:
   ```bash
   grep -q "groundedness" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md
   ```

3. Research domain.json configuration shown with research domain and eval mode:
   ```bash
   grep -q '"domain": "research"' /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md && \
   grep -q "evalMode.*groundedness" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md
   ```

4. File-copy isolation pattern mentioned (same as writing domain):
   ```bash
   grep -q "file-copy" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md && \
   grep -A 100 "## Research Domain Walkthrough" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | \
   grep -q "file-copy"
   ```

5. Section appears after Writing Domain Walkthrough:
   ```bash
   WRITING_LINE=$(grep -n "## Writing Domain Walkthrough" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | cut -d: -f1) && \
   RESEARCH_LINE=$(grep -n "## Research Domain Walkthrough" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | cut -d: -f1) && \
   test "$RESEARCH_LINE" -gt "$WRITING_LINE"
   ```

## Regression Evals (full test suite)

This is a documentation-only task. No automated test suite applies. Manual content review in Auditor phase.

## Acceptance Checks (verification commands)

1. Verify Research section has substantive content (min 10 lines):
   ```bash
   sed -n '/## Research Domain Walkthrough/,/^## /p' /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | wc -l | awk '{if ($1 > 10) exit 0; else exit 1}'
   ```

2. Verify cross-references to architecture docs are valid:
   ```bash
   grep -o "domain-adapters\.md" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md && \
   test -f "/Users/danleemh/ai/claude/evolve-loop/docs/domain-adapters.md"
   ```

3. Verify no broken internal links in Research section:
   ```bash
   sed -n '/## Research Domain Walkthrough/,/^## /p' /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | \
   grep -oP '\[.*?\]\(\K[^)]+' | while read link; do \
     test -f "/Users/danleemh/ai/claude/evolve-loop/$link" || exit 1; \
   done
   ```

## Thresholds

- All checks: pass@1 = 1.0
- Expected execution: 5 graders, all passing
