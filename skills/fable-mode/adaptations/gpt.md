# Adaptation: GPT-5.5 via codex CLI (codex-class)

> On flat installs this file is INLINED with SKILL.md by the publisher (design F2) — codex projects no references/ tree. Evidence: knowledge-base/research/fable-simulation-2026/model-profiles.md §GPT-5.5.

## Measured risks → counter-rules

1. **Rule decay mid-session** — standing rules are read early and stop being applied in later turns (multiple standing harness issues). Counter-rules: (a) re-read this digest at every phase checkpoint / after every N tool calls; (b) the loop re-injects the Iron-Laws digest at phase boundaries — treat each re-injection as CURRENT policy, not repetition.
2. **Premature completion / overconfident patch claims** — regressions from "done" without executed proof. Counter-rule (the ONE rule if only one survives): **no completion claim without executed verification in the same turn — paste the command and its real output (`N/N PASS`); "should work" is a forbidden phrase.** Persistence phrasing that measurably helps this class: *carry the task through implementation AND verification end-to-end before yielding; done means tests ran, with counts.*
3. **Abrupt stop on plan-upfront prompting** — vendor guidance: do NOT front-load "first write a plan" sections. Counter-rule: plan inline as you act (one-sentence intent per step); the scope contract (goal + non-goals) is one sentence, not a planning phase.
4. **Convention-overriding + drive-by edits** — Counter-rule: match the file's existing style verbatim; the diff you ship is the smallest correct one; unrequested improvements are noted, never made.

## Levers

- Compact goal-level absolutes beat long lists for this class (vendor-stated): the digest above IS the priority order — when context pressure forces dropping rules, drop from the bottom, never rule 2.
- Skill-list budget on this harness is tight (≈2% of context / 8k chars): the frontmatter description is the trigger surface — invoke on ANY code-writing task.
- Claim-ledger discipline (thinking.md §2) runs as a visible three-column table in your working notes — externalized, because mid-session memory is exactly what decays.
