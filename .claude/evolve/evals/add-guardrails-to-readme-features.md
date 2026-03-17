# Eval: add-guardrails-to-readme-features

## Task
Add denial-of-wallet guardrails to the README Features section. Currently the README lists 7 features but omits the guardrails added in v4.2.0.

## Current README Features section
```
- **4 specialized agents** — Scout, Builder, Auditor, Operator
- **5 lean phases** — DISCOVER → BUILD → AUDIT → SHIP → LEARN
- **Multi-task per cycle** — 2-4 small tasks built and audited sequentially
- **Worktree isolation** — Builder works in isolated git worktrees
- **Eval hard gate** — Auditor runs code graders and acceptance checks before shipping
- **Continuous learning** — instinct extraction after each cycle with deep reasoning
- **Loop monitoring** — Operator detects stalls, quality degradation, and repeated failures
- **No external dependencies** — fully self-contained Claude Code plugin
```

## Change
Add after "Loop monitoring" bullet:
```
- **Denial-of-wallet guardrails** — configurable cycle cap and cost warnings prevent runaway sessions
```

## Acceptance Criteria

### Code Graders

1. **Guardrails feature listed in README**
   ```bash
   grep -c "Denial-of-wallet guardrails" README.md
   # expected: 1
   ```

2. **Feature appears in Features section (before Architecture section)**
   ```bash
   grep -n "Denial-of-wallet\|## Architecture" README.md
   # expected: Denial-of-wallet line number < Architecture line number
   ```

3. **Feature count is now 8 (add 1 to existing 7 — just verify the new line is present)**
   ```bash
   grep -c "\*\*.*guardrails\*\*" README.md
   # expected: 1
   ```

### Acceptance Checks

4. **Wording is concise** — The new bullet is one line, matches the style of existing bullets (bold label followed by em dash and description).
