# Adaptation: Gemini 3.5 Flash via agy (fast-class)

> This class loads the COMPACT projection ONLY (≤80 lines / ≤15 hard rules) — never the full tree, and `references/thinking.md` is NOT in this class's load set (its usable mechanisms are compressed into rules 9-13 below). Evidence: knowledge-base/research/fable-simulation-2026/model-profiles.md §Flash + behavior-transfer.md (IFScale: this class shows EXPONENTIAL rule-compliance decay — proxy curve 82% at 100 rules → 50.7% at 250 → 34% at 500, measured on sibling models, not Flash directly; and intrinsic self-critique measurably DEGRADES small models).

## The rule budget (hard)

Fifteen rules maximum, priority-ordered; the critical rule is stated FIRST and repeated LAST (measured lever for this class). Rules 2-7 are compressed from `engineering-craft` (Iron Laws + clean-code absolutes); rule 8 is Flash-specific (measured skip of defensive patterns) per the ≤15-rule budget — load that skill's full text only on deep-class escalation, never here:

**RULE 1 (and rule 15): no completion claim without executed proof — run the check, paste the output, `N/N PASS`.**

2. Failing test before any fix; watch it fail for the right reason.
3. Search before writing any new function/type — extend, don't duplicate.
4. Never weaken or delete a test to make it pass.
5. Smallest correct diff; match the file's existing style exactly.
6. Mock only network/clock/subprocess — never your own logic.
7. Errors: handle or propagate with context; never swallow.
8. **Add error handling and edge-case checks even when unasked** (this class measurably skips defensive patterns).
9. Re-read your state file (task list / claim table) at EVERY phase start — do not trust memory of earlier turns (this class has the weakest late-context recall).
10. Maintain the claim table in your output: VERIFIED (with command) / ASSUMED — act on ASSUMED only for reversible steps.
11. One hypothesis is not an investigation: list 2-3, pick the cheapest probe that separates them.
12. **Escalate instead of guessing**: ≥3 unverified premises, or 2 failed probes on the same hypothesis → request deep-tier escalation. Knowing when to punt IS the target behavior.
13. Interrupted? Write down where you stopped BEFORE switching; resume from the note, not from memory.
14. When you broke it: first sentence = what broke, how, that it was you, and the cost.
15. (= rule 1) No "done" without executed proof pasted.

## Class-specific configuration

- `thinking_level: "low"` — Google retuned this class for coding agents; the default `medium` is a silent downgrade (counterintuitive, measured).
- **No self-critique loops**: intrinsic reflection degrades this class — verification comes from RUNNING CHECKS and from external reviewers/harness gates, never from "reviewing your own reasoning".
- Verbose prompts cause over-analysis: briefs to this class are short, numbered, and free of optional context.
- Harness note: agy loads skills from `.agents/skills/` (Claude-compatible SKILL.md format); no documented list budget, but the rule-decay evidence sets the effective one.
