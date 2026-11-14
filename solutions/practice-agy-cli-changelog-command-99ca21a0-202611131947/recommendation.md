# Recommendation — practice-agy-cli-changelog-command-99ca21a0-202611131947

Every number cites `assumptions-and-evidence.md`. The full review is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: bullet, no persona change | Option 2: edit a persona now | Option 3: bridge reads context usage |
|---|---|---|---|
| Persona edits | 0 (E4) | 1 or more (breaks E3) | 0 |
| Baseline for D8 (E3) | not needed | missing, so no gain can be proved | not needed |
| Changelog lines that it addresses (E1) | all, as data | none that a persona states (A3) | ACP context usage only |
| Cost | one trivial document cycle (E6) | persona edit plus D8 cycles (E3) | one build cycle (E6) |
| Fits ADR-0133 | yes | no | yes, with no benefit now |

## Recommendation

**Adopt Option 1: add one note bullet and change no persona.** The deciding facts are E2 and E3. No 1.5.8 line affects a persona rule, and a persona edit needs a measured baseline. The changelog lines (E1) are untrusted data (A2).

**Runner-up:** Option 3, because it adds no prompt bytes. It has no live target today.

**Evidence that flips the choice:** a bridge run that ends at a context limit that the ACP report can show. Then Option 3 gains a live target.

**What this review cannot source:** the exact wording of the full 1.5.8 changelog and any token saving of a persona edit (E6). The text comes from the inbox record only.
