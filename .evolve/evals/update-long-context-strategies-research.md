# Eval: update-long-context-strategies-research

## Code Graders (bash commands that must exit 0)
- `[code]` `test -f docs/private/research/long-context-agent-strategies.md`

## Acceptance Checks
- `[code]` `grep -qi "Lindenbauer" docs/private/research/long-context-agent-strategies.md`
- `[code]` `grep -qi "observation masking" docs/private/research/long-context-agent-strategies.md`
- `[code]` `grep -qi "ACON" docs/private/research/long-context-agent-strategies.md`
- `[code]` `grep -qi "paired trajectory" docs/private/research/long-context-agent-strategies.md`
- `[code]` `grep -qi "Gloaguen" docs/private/research/long-context-agent-strategies.md`
- `[code]` `grep -qi "MECW" docs/private/research/long-context-agent-strategies.md`

## Thresholds
- All checks: pass@1 = 1.0
