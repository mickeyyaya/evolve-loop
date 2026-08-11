# Eval: add-self-evolving-tool-creation-doc

> Graders for the self-evolving tool creation documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/self-evolving-tool-creation.md` | exit 0 |
| 2 | bash | `grep -q "detect\|detection\|need identification" docs/self-evolving-tool-creation.md` | exit 0 |
| 3 | bash | `grep -q "generate\|generation\|synthesize" docs/self-evolving-tool-creation.md` | exit 0 |
| 4 | bash | `grep -q "validate\|validation\|verify" docs/self-evolving-tool-creation.md` | exit 0 |
| 5 | bash | `grep -q "register\|registry\|catalog" docs/self-evolving-tool-creation.md` | exit 0 |
| 6 | bash | `wc -l < docs/self-evolving-tool-creation.md \| awk '{exit ($1 > 400)}'` | exit 0 — under 400 lines |
| 7 | bash | `grep -c "^|" docs/self-evolving-tool-creation.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 8 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/self-evolving-tool-creation.md` | exit 0 |
| 9 | bash | `grep -q "Scout\|Builder\|Auditor" docs/self-evolving-tool-creation.md` | exit 0 — maps to evolve-loop agents |
