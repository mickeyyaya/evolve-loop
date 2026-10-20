# Research: who fixes a defect that the next phase finds

- **Status:** complete, 2026-10-20.
- **Plan:** [finder-fix-ownership-2026-10](../plans/finder-fix-ownership-2026-10.md). **Decision:** [ADR-0146](../architecture/adr/0146-a-defect-that-survives-two-builder-rounds-is-fixed-from-the-finders-spec-and-verified-by-another.md).
- **Related:** [ADR-0142](../architecture/adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md) and its spec [retry-ladder](../architecture/retry-ladder.md).
- **Method:**
  - the run artifacts of cycles 1900 to 1997 (`signals.ndjson`);
  - the auditor profile and persona;
  - live web searches on 2026-10-20 (section 3).

## Table of contents

- [1. The request](#1-the-request)
- [2. Local evidence](#2-local-evidence)
- [3. Current practice](#3-current-practice)
- [4. Limits](#4-limits)

## Terms used throughout

- **Finder:** the phase that finds a defect in the output of the phase before it, for example the auditor in a build.
- **Owner:** the phase whose output holds the defect, for example the builder.
- **Fix spec:** the finder's account of a defect: its location, the expected behavior, and a suggested patch.
- **Verifier:** the check that decides if a fix is correct. It must not be the author of the fix.

## 1. The request

The operator asked on 2026-10-20: "if build agent made mistakes or defects and found by the next phase agent, auditor in this example, after 1 retry for build agent to fix and it still failed, it bumps the effort to the next level (e.g. from medium to high), and if it still failed, I'm leaning to have the phase agent who provides the suggestions to fix the issue/defects directly."

## 2. Local evidence

**F2.1** Cycles 1900 to 1997 had 24 audit repair rounds. In 15 the re-audit passed or warned, in 6 it failed, and 3 had no re-audit. The source is `ORCHESTRATOR_AUDIT_REPAIR_GRANTED` and the audit outcome after it, measured on 2026-10-20.
*Implication:* a repair round of the owner fixes about 71% of the audit failures with a verdict (15 of 21 rounds, first and second). It is worth keeping.

**F2.2** Cycles 1972 and 1983 each had two repair rounds (the cap is two, `attempt 1/2`) and both still failed the audit.
*Implication:* a second round of the same owner adds little. This is the case that the operator's proposal targets.

**F2.3** The auditor runs with `read_only_repo: true` and can write only under `.evolve/runs/cycle-*` (`.evolve/profiles/auditor.json`). Its persona says "READ-ONLY — do not modify source files" (`agents/evolve-auditor.md:21`).
*Implication:* an auditor that edits the code needs a wider sandbox. An auditor that writes a fix spec into its own cycle workspace needs no change.

**F2.4** The judge of a phase handoff must stay independent of the try it grades ([ADR-0141](../architecture/adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md)), and ADR-0142 keeps judge seats off the ladder.
*Implication:* if the finder writes the fix, a different check must grade it.

## 3. Current practice

The searches ran on 2026-10-20. "acc." marks a page with no date of its own.

### Who finds, who writes, who approves

**R1** In Gerrit a reviewer writes the replacement lines with "suggest fix", and the owner of the change applies them ([Gerrit](https://gerrit-review.googlesource.com/Documentation/user-suggest-edits.html), acc.).

**R2** Copilot code review writes suggested changes. "Fix with Copilot" hands the finding to a different agent, and a person merges the result ([GitHub changelog](https://github.blog/changelog/2026-05-19-easily-apply-copilot-code-review-feedback-with-copilot-cloud-agent), 2026-05-19).

**R3** CodeRabbit Autofix gives the review's findings to a coding agent. That agent applies and builds them, and the result is reviewed like other code ([CodeRabbit](https://docs.coderabbit.ai/finishing-touches/autofix), acc.).

**R4** Meta's SapFix checks each patch with deterministic tests before a person, usually the author of the bad change, approves it ([Meta](https://engineering.fb.com/2018/09/13/developer-tools/finding-and-fixing-software-bugs-automatically-with-sapfix-and-sapienz/), 2018).

**R5** GitHub can require that the last push be approved by someone other than its author ([GitHub](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches), acc.). Separation of duties in change control says the same: the author of a change does not approve it ([Liquibase](https://www.liquibase.com/technical-glossary/separation-of-duties), 2025, a vendor source).
*Implication:* no surveyed practice lets the finder both write and approve a fix.

### Roles in multi-agent coding

**R6** AgentCoder keeps the tests independent and deterministic, and the programmer fixes what they find. It reports 96.3% pass@1 on HumanEval with GPT-4 ([arXiv 2312.13010](https://arxiv.org/abs/2312.13010), 2024).

**R7** In Aider, an architect model plans the change and an editor model applies it. R1 as architect with Sonnet as editor scored 64.0% on the polyglot benchmark ([Aider](https://aider.chat/2025/01/24/r1-sonnet.html), 2025-01-24). Anthropic's advisor gives guidance and never edits ([Anthropic](https://claude.com/blog/the-advisor-strategy), 2026-04-09).
*Implication:* "one role plans the fix, another applies it" is a working pattern.

### Bias and the value of the finder

**R8** A model that judges its own output favors it. The bias grows with how well it recognizes its own text ([arXiv 2404.13076](https://arxiv.org/abs/2404.13076), NeurIPS 2024). Self-bias grows over rounds of self-refinement ([ACL 2024](https://aclanthology.org/2024.acl-long.826)).
*Implication:* the finder cannot grade its own fix.

**R9** Models fail to locate their own errors, but they correct them well once they get the location ([arXiv 2311.08516](https://arxiv.org/abs/2311.08516), Findings of ACL 2024). In code self-repair, the quality of the feedback is the bottleneck ([arXiv 2306.09896](https://arxiv.org/abs/2306.09896), ICLR 2024).
*Implication:* the finder's most valuable output is the exact location and a clear account of the defect.

### Retries and other agents

**R10** Most models lose 60% to 80% of their debugging effect within 2 to 3 attempts in one context ([arXiv 2506.18403](https://arxiv.org/abs/2506.18403), 2025, a preprint).

**R11** Different agents resolve different issues. A committee of agents resolved about twice the distinct issues of one agent ([arXiv 2408.07060](https://arxiv.org/abs/2408.07060), ICLR 2025). Different models still fail on the same hard parts of a specification ([arXiv 2606.20158](https://arxiv.org/abs/2606.20158), 2026).
*Implication:* a fresh agent, and better a different family, helps, but it is not full independence.

**R12** Anthropic calls effort "a behavioral signal, not a strict token budget", and advises a sweep on your own evals ([effort docs](https://platform.claude.com/docs/en/build-with-claude/effort), read 2026-10-20). No source found compares an effort raise with a change of agent as a fix for a repeated failure.
*Implication:* the effort rung must record its result, so the loop learns if it helps.

## 4. Limits

- No study measures if a critic loses its power to detect once it also edits. The case for separation rests on R5 and R8.
- Several sources were read only as abstracts or vendor pages (R3, R5, R10).
- The local counts of F2.1 are 24 rounds. They show the shape, not a precise rate.
