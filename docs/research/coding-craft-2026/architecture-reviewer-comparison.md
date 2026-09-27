# Architecture reviewer: comparison with popular review prompts (2026-09-28)

The operator asked for the evo-loop architecture reviewer (`.claude/agents/architecture-reviewer.md`) to judge whether a change strictly follows test-driven development, *Clean Code* and *Design Patterns*, and to be compared with the most popular review prompts online. This records how the comparison was run, what each contestant did, and what the reviewer adopted.

## Question

Which reviewer prompt best catches what the project's standard forbids, without asking for changes a principal engineer would not request? The standard is the four files the reviewer judges against: `skills/engineering-craft/references/tdd-craft.md`, `clean-code.md`, `design-patterns.md` and `docs/conventions/code-comments.md`.

## Method

The comparison follows the writing-skills discipline: a failing baseline first, then the change, then a re-test.

- **Three scenarios**, each a small Go module with one uncommitted change and the author's verification text:
  - **Seeded violations.** A strike-and-bench feature with fourteen planted violations: behavior with no test, two tests that cannot fail, a bug fix without its regression test, a function nested five deep doing six jobs, a flag argument and five parameters, seven new comments (one narrating a cycle), a query that deletes, an unused five-method Strategy, an embedded type that panics where its base returns, dead helpers, two magic numbers, a new environment toggle, a swallowed write error and the family-key rule written three times.
  - **Subtle gaps.** The same feature built test-first: six behavior tests, a recorded red run against a stub, five killed mutants. Two behaviors are still unpinned (per-family strike counts; a strike during a bench not counting toward the next), each surviving a mutant.
  - **Clean.** The subtle scenario with those two tests added (eight tests, seven killed mutants). The judges found it still had real gaps (see Results), which is itself a finding about how hard "clean" is to reach.
- **Contestants.** Each ran as a fresh agent on the same model, with the same inputs, including the paths of the project standard:
  - the reviewer on main (the baseline), once with and once without the standard's paths;
  - the draft that added the three-book audit (v1);
  - the five highest-ranked online prompts, chosen from 35 candidates across 32 repositories by a quality rubric weighted by adoption.
- **Online contestants:**

  | Prompt | Source | Stars at retrieval |
  |---|---|---|
  | code-reviewer | obra/superpowers, `skills/requesting-code-review/code-reviewer.md` | 292K |
  | code-review-and-quality | addyosmani/agent-skills, `skills/code-review-and-quality/SKILL.md` | 99K |
  | code-reviewer | affaan-m/ECC, `agents/code-reviewer.md` | 268K |
  | architecture-critic | anthropics/claude-plugins-official, `plugins/code-modernization/agents/architecture-critic.md` | 37K |
  | code-review | mattpocock/skills, `skills/engineering/code-review/SKILL.md` | 270K |

- **Blind judging.** One judge agent per scenario read the anonymized reports (letters only), the diff and the oracle. It scored:
  - recall against the seeded violations, marking a finding with a rejected fix (for a comment, rewording instead of deleting) as a wrong fix;
  - factually wrong claims;
  - false alarms at HIGH or above;
  - whether the verdict was right;
  - how actionable each fix was.

  The judges verified claims by running the mutants and probes themselves.

## Results

Round one. Rank 1 is best in each scenario.

| Contestant | Seeded | Subtle | Clean | What decided it |
|---|---|---|---|---|
| Draft v1 (three-book audit) | 1 (14/14) | 2 | 2 | Found everything, and was the only report whose CRITICAL tier was exactly the defects that matter. It inflated test-only gaps over correct code to CRITICAL/Block. |
| Baseline with the standard's paths | 2 (14/14) | 1 | 8 | Strong on the planted and subtle gaps. Approved the clean change and claimed every behavior was pinned when two were not. |
| architecture-critic | 3 (14/14) | 7 | 5 | Strongest evidence (mutants as the acceptance bar, probes). Raised Policy validation at HIGH, a false alarm. |
| code-review-and-quality | 4 (14/14) | 6 | 1 | Best on the clean change. Missed one subtle gap. |
| ECC code-reviewer | 5 (13/14) | 4 | 4 | Precise, with zero noise. Approved while writing "worth adding before merge". |
| superpowers code-reviewer | 6 (13/14) | 3 | 3 | Its "Declined to judge" list kept speculation out of its findings. Also raised Policy validation as a merge condition, a false alarm. |
| Baseline as-is, no standard | 7 (10/14) | 8 | 6 | Missed the test-driven violations and asked for a reworded comment. |
| mattpocock code-review | 8 (0 by rubric) | 5 | 7 | Detected 13 of 14 in its smell catalogue, but its output has no fix, severity or verdict. |

**Two findings shaped the rewrite:**

- **The draft's lead comes from procedure, not knowledge.** The baseline given the same standard files came second on seeded violations and first on subtle gaps. It came last on the clean change, because it did not run anything. Every contestant that ran mutants found gaps the others missed.
- **The best techniques were spread across prompts:**
  - a control mutant before the sweep, proving the harness detects failures (addyosmani);
  - a red run counts only when the named assertion fails, not a setup line (architecture-critic, mattpocock, addyosmani);
  - mutant classes a deletion misses: reorder, shift, wrong key, wrong tense, a hard-coded value equal to the tests' value (the baseline with the standard; addyosmani);
  - validation is requested only where a caller can produce the bad input (ECC, addyosmani);
  - a "Declined to judge" list (superpowers);
  - a single most-important change (architecture-critic).

Round two compared v1 with v2, which folds those techniques in:

| Scenario | v1 | v2 |
|---|---|---|
| Seeded | 14/14 | 13/14. Its regression test for the bug fix passed on the pre-fix code, and it claimed two kills it had not run. |
| Subtle | Both gaps, rated CRITICAL | One gap plus three further real survivors (a reset to −1, an "ever benched" guard, a hard-coded hour), all correctly at HIGH. Missed the shared counter. |
| Clean | All four real gaps; CRITICAL/Block and a LOW for the standard's own example | Three real gaps, correctly HIGH with FIX_THEN_MERGE. Missed the raw-spelling counter. |

v2's two misses were wrong-key mutants it skipped as "more than one line", and its seeded-scenario slip was prescribing tests it had not run. The refactor that followed made three changes:
- A mutant is one idea, not one line: a wrong-key mutant changes every use of the key, and each map or field the change writes gets its own.
- A kill or a regression test the reviewer prescribes must have been run: the kill against its mutant, the regression test on the pre-fix code.
- A bug fix without its regression test is its own CRITICAL finding.

The refactored profile was re-run on the two scenarios where v2 had slipped:

| Scenario | Re-check result |
|---|---|
| Seeded | The bug fix is its own CRITICAL finding. Its regression test was run and failed on the pre-fix code for the named reason; the existing test stays as the preservation side. Ten mutants, with the harness proven by a control first: the author's tests killed none; the reviewer's proposed tests killed nine, each run. The tenth is the environment toggle, whose fix is deletion. |
| Subtle | Six survivors, all verified: both real gaps (the shared counter and the strike during a bench), plus a reset to −1, an "ever benched" guard, a raw-spelling counter and a hard-coded hour. All at HIGH with FIX_THEN_MERGE, and the production code is stated correct. |

## What the reviewer adopted

Every item traces to a judged report. Each is in `.claude/agents/architecture-reviewer.md` or `tdd-craft.md` §Reviewing for TDD:

- **Mutation sweep.** Up to ten single-idea mutants of the changed code in a scratch copy, after a control mutant. The report lists survivors and equivalents separately.
- **Red-first scrutiny.** A red run counts only when the assertion the test is named for fails.
- **Calibration.**
  - An unpinned change and a bug fix without its regression test are CRITICAL.
  - A surviving mutant of otherwise tested, correct code is HIGH with FIX_THEN_MERGE.
  - This removes v1's Block on test-only gaps without losing the gap.
- **Noise control:**
  - No guard or validation request where nothing in the repository can produce the bad input.
  - No design question without a requirement as a finding; both go under "Declined to judge".
  - Every behavior claim is verified by a probe, a mutant or traced lines.
- **Output contract.** The three-book audit carries the sweep's counts. New sections: "Declined to judge" and "The one change".

## Limits

- The scenarios are small and single-package. Behavior on a large diff is governed by the reviewer's budget rule (the riskiest seams first, with what was skipped named), which this comparison did not exercise.
- Each contestant ran once per scenario, so a one-rank difference is within noise. The draft's consistent top-two placement across all three scenarios is the signal.
- The online prompts ran with this project's standard supplied. Without it, most would have missed the comment and zero-flag rules, which are local conventions.
