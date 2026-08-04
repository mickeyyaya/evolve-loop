# Coding-Discipline Skills for LLM Agents — Ecosystem Research (2025–2026)

Researched 2026-07-07. Sources: web search + WebFetch, plus **primary-source inspection of locally installed plugins** (superpowers 6.1.1 and ECC 2.0.0 in `~/.claude/plugins/cache/`). Star counts are as reported by the cited pages on their crawl dates; treat as approximate.

---

## 0. Verdicts on the two named references

- **superpowers** = [obra/superpowers](https://github.com/obra/superpowers) — Jesse Vincent (Prime Radiant). Confirmed; the dominant skills framework. ~150k stars in Apr 2026, ~245k stars / 21.7k forks by early July 2026; accepted into the official Anthropic Claude Code marketplace 2026-01-15.
- **ponytail** = [DietrichGebert/ponytail](https://github.com/DietrichGebert/ponytail) — **it exists and is exactly what the name suggests the user meant**: a viral (mid-2026) minimalism/anti-over-engineering skill: "Makes your AI agent think like the laziest senior dev in the room. The best code is the code you never wrote." ~75.8k stars per [SkillsLLM](https://skillsllm.com/skill/ponytail). It is NOT a misremembering of something else.

---

## 1. obra/superpowers — the reference framework

**Repo / activity / adoption.** [github.com/obra/superpowers](https://github.com/obra/superpowers). Extremely active: v6.1.0 released 2026-06-30, v6.1.1 on 2026-07-02 (verified locally in `RELEASE-NOTES.md`). Multi-harness: Claude Code, Codex (app+CLI), Cursor, Antigravity, Factory Droid, Copilot CLI, Kimi Code, OpenCode, Pi. Satellite repos: [superpowers-marketplace](https://github.com/obra/superpowers-marketplace), [superpowers-skills](https://github.com/obra/superpowers-skills) (community-editable), [superpowers-lab](https://github.com/obra/superpowers-lab) (experimental). "Most-used Claude Code plugin in the world" per author. Story: [blog.fsck.com/2025/10/09/superpowers](https://blog.fsck.com/2025/10/09/superpowers/), [Superpowers 4](https://blog.fsck.com/2025/12/18/superpowers-4/).

**Structure (from local 6.1.1 install).** 14 core skills, ~3,300 lines total; each `skills/<name>/SKILL.md` with minimal YAML frontmatter (`name` + `description` only). Skills: brainstorming, writing-plans, executing-plans, subagent-driven-development, test-driven-development, systematic-debugging, requesting/receiving-code-review, verification-before-completion, finishing-a-development-branch, using-git-worktrees, dispatching-parallel-agents, writing-skills, using-superpowers. Conventions (from `writing-skills/SKILL.md`):
- **Description = triggering conditions ONLY, never workflow summary.** Their testing found that a description which summarized the workflow ("code review between tasks") caused the agent to follow the description and skip the skill body (did 1 review instead of the flowchart's 2). This is a measured, load-bearing finding.
- **Graphviz dot-digraph flowcharts** only where an agent "might go wrong" at a decision point; never for linear steps (style guide in `graphviz-conventions.dot`).
- **Token budgets**: bootstrap skills <150–200 words, others <500; v6.1.0 explicitly re-compressed the bootstrap ("replaced the graphviz skill-flow diagram with the prose it encoded") to cut per-session cost.
- Progressive disclosure: heavy reference (100+ lines) and reusable tools go in separate files; principles stay inline.
- Rigid-vs-flexible is expressed per failure mode: "prohibition + rationalization table + red flags" for discipline failures; softer forms for shaping-output failures ("Match the Form to the Failure").

**Coding-discipline content.** `test-driven-development/SKILL.md` (371 lines) is the archetype:
- **Iron Law**: `NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST`; wrote code first? **"Delete it. Start over."** — explicitly forbids keeping it "as reference" or "adapting" it.
- Red-Green-Refactor as a dot digraph with verify-RED and verify-GREEN diamond gates; verify-RED requires the test to *fail for the right reason* (feature missing, not typo/error).
- Test-quality rules: one behavior per test, name describes behavior, real code over mocks; Good/Bad paired code examples (`<Good>`/`<Bad>` tags).
- An 11-row **Common Rationalizations table** ("Too simple to test" → "Simple code breaks. Test takes 30 seconds."), a 13-item **Red Flags list** ("This is different because..."), a per-completion **verification checklist** ("Can't check all boxes? You skipped TDD. Start over."), and pre-rebutted essays against "tests-after achieves the same goals" and sunk-cost arguments.
- `systematic-debugging`: Iron Law `NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST`; 4 phases, Phase 1 = read errors/reproduce/check recent changes/instrument component boundaries; explicitly targets time-pressure rationalization.
- `verification-before-completion`: Iron Law `NO COMPLETION CLAIMS WITHOUT FRESH VERIFICATION EVIDENCE`; a 5-step "Gate Function" ("Skip any step = lying, not verifying"); table mapping each claim type to required evidence ("Agent completed" requires VCS diff, not the agent's success report).
- Code review is split into requesting vs receiving skills; the full lifecycle chains brainstorm → plan → worktree → TDD → subagent execution → review → finish-branch.

**Enforcement mechanisms.**
1. **SessionStart hook** (`hooks/hooks.json` → `session-start` script) injects the entire `using-superpowers` skill wrapped in `<EXTREMELY_IMPORTANT>` into every session (startup/clear/compact) — the only hard mechanism; everything else is language-level.
2. `using-superpowers` mandatory-invocation language: "If you think there is even a 1% chance a skill might apply… you ABSOLUTELY MUST invoke the skill… This is not negotiable. You cannot rationalize your way out of this." Plus a 12-row red-flags table of rationalizing thoughts ("This is just a simple question" → "Questions are tasks. Check for skills.").
3. **"Violating the letter of the rules is violating the spirit of the rules"** appears in each discipline skill — cuts off "spirit-not-ritual" loopholes.
4. **Skill development is itself TDD**: `writing-skills` mandates baseline **pressure-scenario testing with subagents** (RED = agent violates without skill; document verbatim rationalizations; GREEN = complies with skill; REFACTOR = plug new loopholes), combined pressures (time + sunk cost + exhaustion), and cheap "micro-tests" for wording before full pressure runs.
5. Contributor governance as enforcement: repo `CLAUDE.md` reports a **94% PR rejection rate** and requires eval evidence for any change to behavior-shaping content ("PRs that restructure skills to 'comply' with Anthropic's skills documentation will not be accepted without extensive eval evidence"). Notably: their skill philosophy *deliberately diverges* from Anthropic's published guidance, on empirical grounds.

**Effective vs decorative.** The project's own methodology is eval-driven (baseline-vs-skill subagent pressure tests; the description-summarization trap was found by testing). Public quantitative outcome data is thin — adoption, longevity, and the maintainers' internal evals are the main signals. Community consensus (awesome-lists, [ClaudeFast comparison](https://claudefa.st/blog/tools/skills/best-claude-code-skills)) treats it as the benchmark; the framework "will literally delete code written before tests exist."

---

## 2. DietrichGebert/ponytail — minimalism discipline ("the lazy senior dev")

**Repo / activity / adoption.** [github.com/DietrichGebert/ponytail](https://github.com/DietrichGebert/ponytail). 2026 newcomer, viral: ~75.8k stars ([SkillsLLM](https://skillsllm.com/skill/ponytail)), Trendshift-ranked, coverage on [DEV](https://dev.to/yashddesai/ponytail-the-ai-coding-skill-taking-github-by-storm-and-the-one-question-nobodys-answered-yet-46mc) and [Medium (Joe Njenga, Jun 2026)](https://medium.com/@joe.njenga/i-tried-claude-code-ponytail-and-found-the-lazy-solution-youve-been-missing-d0828735c8bb). Adapters for **16 agents** (Claude Code, Codex, Copilot CLI, Cursor, Windsurf, OpenCode, Gemini CLI, Devin CLI, Cline…); MIT.

**Structure.** Core skill is **~100 lines of Markdown** ([skills/ponytail/SKILL.md](https://github.com/DietrichGebert/ponytail/blob/main/skills/ponytail/SKILL.md)); the rest of the repo is multi-agent adapter infrastructure (`AGENTS.md`, `.cursor/rules/`, `.windsurf/rules/`, hooks). Skill-capable hosts get commands; instruction-only hosts (Cursor/Windsurf/Cline) load an always-on ruleset. Companion commands: `/ponytail-review` (audit diffs), `/ponytail-audit` (full repo), `/ponytail-debt` (deferred shortcuts), `/ponytail-gain` (impact scorecard).

**Coding-discipline content — "The Lazy Ladder"** (stop at the first rung that works):
1. Does the task need to exist? (YAGNI) → 2. Already in this codebase? → 3. Stdlib covers it? → 4. Native platform feature? → 5. Already-installed dependency? → 6. One line? → 7. Minimum viable code.
Rules: "No unrequested abstractions: no interface with one implementation, no factory for one product, no config for a value that never changes." Deletion over addition; boring over clever; fewest files; deliberate simplifications marked with `ponytail:` comments naming the ceiling and upgrade path. Guardrail: "Lazy means writing less code, not picking the flimsier algorithm" — never simplify away validation, error handling, security, accessibility, or requested features.

**Enforcement.** Intensity levels (`lite` / `full` default / `ultra` / `off`); always-on ruleset injection on non-skill hosts; review/audit commands as after-the-fact gates. No hard blocking — it is persona + ladder + audit.

**Measured effectiveness (best-in-ecosystem benchmark honesty).** Headless Claude Code sessions on a real FastAPI+React repo, 12 feature tasks, Haiku 4.5, n=4, vs no-skill baseline: **−54% LOC (mean; up to −94% on over-build cases), −22% tokens, −20% cost, −27% time, 100% safety-checks preserved**. Notably the README flags that its older "80–94% less code" single-shot number was partly a conversational-baseline artifact and publishes the corrected agentic numbers — rare methodological honesty in this space.

---

## 3. anthropics/skills — the official repo + spec

**Repo / activity / adoption.** [github.com/anthropics/skills](https://github.com/anthropics/skills). ~149k stars / 17.6k forks (Jun 2026), active commits. Agent Skills became an **open standard** ([agentskills.io](https://agentskills.io), Dec 2025) adopted by ~40 clients (Copilot, VS Code, Cursor, Codex, Gemini CLI, Goose, OpenCode…). Anthropic engineering rationale: [Equipping agents for the real world with Agent Skills](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills).

**Structure.** Canonical layout `skill-name/{SKILL.md, references/, scripts/, assets/, evals/}`; frontmatter requires only `name` + `description`; **progressive disclosure** doctrine: metadata always visible (~100 words) → SKILL.md body loaded on demand (<500 lines) → references/scripts loaded as needed. Spec: [spec/agent-skills-spec.md](https://github.com/anthropics/skills/blob/main/spec/agent-skills-spec.md). Authoring best practices: [platform.claude.com docs](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices).

**Coding-discipline content.** Thin by design — 17 skills, mostly documents/creative/enterprise ([guide](https://claude-world.com/articles/anthropic-official-skills-complete-guide/)). Technical ones: `webapp-testing` (Playwright, "reconnaissance-then-action"), `mcp-builder` (requires a formal 10-question eval), `skill-creator` (baseline-vs-skill comparison methodology), `claude-api`. **No TDD/clean-code/review discipline skills** — that space is entirely community-owned (and superpowers explicitly rejects Anthropic's authoring style for discipline content).

**Enforcement.** None beyond description-based triggering; official position is capability-teaching, not discipline. The `evals/` directory convention and Skills 2.0 (below) are the effectiveness story.

---

## 4. affaan-m/everything-claude-code (ECC)

**Repo / activity / adoption.** [github.com/affaan-m/everything-claude-code](https://github.com/affaan-m/everything-claude-code) (also badged as `affaan-m/ECC`). README claims **182K+ stars, 28K+ forks, 170+ contributors**; Anthropic (Cerebral Valley) hackathon winner, Feb 2026; npm `ecc-universal` / `ecc-agentshield`; v2.0.0 current (verified locally). Cross-harness: Claude Code, Codex, Cursor, OpenCode, Gemini, Zed, Copilot. 10 translated READMEs.

**Structure (from local 2.0.0 install).** Huge: 40+ general skills + ~200 domain skills, 13+ agents, hooks, rules layer (`rules/common/*` digest pattern), commands-as-shims. Frontmatter adds `origin: ECC`. Skills declare explicit **scope boundaries** ("Do not use this skill as the primary source for…") and cross-reference narrower skills — a dispatch-tree organization (`skill-catalog` master index).

**Coding-discipline content.**
- `tdd-workflow`: tests-before-code, 80%+ coverage (unit+integration+E2E), and a distinctive **git-checkpoint evidence protocol**: one commit for "failing test added and RED validated," one for "minimal fix applied and GREEN validated," optional refactor commit; checkpoints must be reachable from current HEAD on the active branch (anti-gaming: no borrowing old commits as evidence).
- `coding-standards`: readability-first, KISS/DRY/YAGNI, immutability defaults, error-handling expectations — explicitly "the shared floor, not the framework playbook."
- `verification-loop`: phased build → typecheck → lint → tests with "If build fails, STOP" gates.
- Language-specific reviewer agents (go/python/ts/rust/java/react…) with "MUST BE USED for X projects" descriptions.

**Enforcement.** Hooks + **hookify** (generate hooks from observed conversation failures); **instincts/continuous-learning v2**: sessions distill into atomic "instincts" with confidence scores, project-scoped vs global, `/instinct-status|import|export`, `/evolve` clusters instincts into skills. Enforcement is layered: prompt-level skills + deterministic hooks + learned instincts.

**Effective vs decorative.** No published benchmark; adoption + hackathon pedigree + the checkpoint-evidence design (which anticipates agents faking TDD) are the credibility signals. Analysis: [apiyi.com deep-dive](https://help.apiyi.com/en/everything-claude-code-plugin-guide-en.html).

---

## 5. nizos/tdd-guard (→ Probity) — hard enforcement, not prose

**Repo.** [github.com/nizos/tdd-guard](https://github.com/nizos/tdd-guard) — 2.2k stars, 172 forks, v1.7.0 (2026-06-23). Author's writeup: [nizar.se/tdd-guard-for-claude-code](https://nizar.se/tdd-guard-for-claude-code/). Successor: **Probity** ("same TDD enforcement, now for Claude Code, Codex, and Copilot CLI, more reliable validation, no test reporters to set up" — repo header says new projects should start there).

**Mechanism — the deterministic pole of the enforcement spectrum.** A **PreToolUse hook** intercepts Write/Edit/MultiEdit; a validator model (configurable speed/capability) checks the change against captured test-run state and **blocks**: (1) implementation without a failing test, (2) implementation beyond current test requirements (over-implementation), (3) adding multiple tests at once. Lint integration drives the refactor step. Reporters for Vitest/Jest/pytest/PHPUnit/Go/cargo/RSpec/Minitest feed real test results in.

**Significance.** This is the counterpoint to superpowers' thesis: instead of bulletproofing prose against rationalization, remove the agent's ability to act on the rationalization. Cost: per-edit validator latency/tokens + test-reporter setup (the pain Probity removes). Docs: [enforcement.md](https://github.com/nizos/tdd-guard/blob/main/docs/enforcement.md).

---

## 6. EveryInc/compound-engineering-plugin

**Repo.** [github.com/EveryInc/compound-engineering-plugin](https://github.com/EveryInc/compound-engineering-plugin) — Kieran Klaassen / Dan Shipper (Every). 21k+ stars, 1.5k forks (Jun 2026, up from ~9.3k in Feb). 37 skills + 51 agents. Guide: [every.to/guides/compound-engineering](https://every.to/guides/compound-engineering); [interview](https://creatoreconomy.so/p/how-to-make-claude-code-better-every-time-kieran-klaassen).

**Discipline model.** Five-stage loop: brainstorm → plan → work → review → **compound** (capture learnings into docs Claude reads next time). Doctrine: "80% planning and review, 20% execution"; "each unit of engineering work should make subsequent units easier." Parallel research subagents before code; review agents for security/architecture/quality with priority triage. Enforcement is workflow-shaped (stage gating) rather than rule-shaped; its distinctive discipline is the mandatory learning-capture step. Adoption signal: runs five real Every products with ~1-person eng teams.

---

## 7. ramziddin/solid-skills

**Repo.** [github.com/ramziddin/solid-skills](https://github.com/ramziddin/solid-skills) — "AI agent skill for writing senior-engineer quality code through SOLID principles, TDD, and clean architecture." Primarily TypeScript/NestJS-flavored, applicable to any OO codebase.

**Structure/content.** One core `skills/solid/SKILL.md` + a `references/` directory with per-topic files: solid principles, TDD (Red-Green-Refactor, tests-before-code), testing, clean code (meaningful names, small functions, "no comments needed"), code smells, design patterns, architecture, object design, complexity — classic progressive-disclosure with the SKILL.md as dispatcher. Triggered for "writing any code, refactoring, planning architecture, reviewing, debugging, creating tests, design decisions." Enforcement is soft (guidance + review checklists); no hooks, no rationalization tables. Representative of the mid-tier: solid content, weaker compliance engineering than superpowers.

---

## 8. multica-ai/andrej-karpathy-skills

**Repo.** [github.com/multica-ai/andrej-karpathy-skills](https://github.com/multica-ai/andrej-karpathy-skills) — ~144k stars per [Firecrawl's roundup](https://www.firecrawl.dev/blog/best-claude-code-skills). Encodes Karpathy's posted guidelines as four behavioral principles: **think before coding** (state assumptions, push back on wrong approaches), **simplicity first** (no premature abstraction), **surgical changes** (minimal diffs), **goal-driven execution with verification loops**. Notable as the "philosophy-as-skill" genre; the user's own global CLAUDE.md "12 Core Agent Rules" (rules 1–4 credited to Karpathy) descends from the same source material. Enforcement: prompt-level only.

---

## 9. mattpocock/skills — "Grill Me" et al.

**Repo.** [github.com/mattpocock/skills](https://github.com/mattpocock/skills) — ~87.3k stars (Firecrawl). Matt Pocock (Total TypeScript). Discipline-relevant: **Grill Me** — "interviews you relentlessly about every aspect of a plan until shared understanding is reached, before any code is written" (pre-code requirements discipline, the superpowers-brainstorming niche as a standalone); **Handoff** — compresses sessions into structured markdown for cross-agent continuation. Enforcement: conversational protocol; the skill refuses to proceed to code while open questions remain.

---

## 10. trailofbits/skills — security/code-audit discipline

**Repo.** [github.com/trailofbits/skills](https://github.com/trailofbits/skills) — professional security firm's collection: static-analysis discipline with CodeQL, Semgrep, variant analysis; property-based testing; audit workflows. The most credible "review standards" content in the ecosystem because it encodes a real audit firm's methodology (tools + procedure, not vibes). Enforcement: procedure + tool invocation (findings must come from tool output, not model intuition).

---

## 11. vercel-labs/agent-skills — rule-pack genre

**Repo.** [github.com/vercel-labs/agent-skills](https://github.com/vercel-labs/agent-skills). Web Design Guidelines (audits UI code against 100+ a11y/UX rules), React Best Practices (57 performance rules, prioritized by impact), Composition Patterns (compound components over boolean-prop proliferation). Represents the "numbered rule-pack + audit pass" structure: enforceable because each rule is checkable against the diff. First-party vendor skills (Vercel, Remotion, Firecrawl, Corey Haines marketing) are the fastest-growing category in 2026.

---

## 12. Aggregators, marketplaces, and honorable mentions

- [hesreallyhim/awesome-claude-code](https://github.com/hesreallyhim/awesome-claude-code) — the canonical curated list (skills, agents, plugins, hooks).
- [travisvn/awesome-claude-skills](https://github.com/travisvn/awesome-claude-skills) (~13k stars), [ComposioHQ/awesome-claude-skills](https://github.com/ComposioHQ/awesome-claude-skills), [GetBindu/awesome-claude-code-and-skills](https://github.com/GetBindu/awesome-claude-code-and-skills), [rohitg00/awesome-claude-code-toolkit](https://github.com/rohitg00/awesome-claude-code-toolkit) (135 agents / 35 skills / 176+ plugins).
- Registries: [skills.sh](https://skills.sh/), [skillsllm.com](https://skillsllm.com/), [awesomeclaudeskills.com](https://awesomeclaudeskills.com/), [claudepluginhub.com](https://www.claudepluginhub.com/), aitmpl.com.
- 2026 newcomers in the token/context-discipline adjacent space: **Caveman** ([JuliusBrussee/caveman](https://github.com/JuliusBrussee/caveman), −65% output tokens claim), **Context Mode** ([mksglu/context-mode](https://github.com/mksglu/context-mode)).
- Official Anthropic marketplace plugins: `code-review`, `code-simplifier` ("readability only, no logic changes"), `feature-dev`, `frontend-design` — small, single-purpose, no hard enforcement.

---

## 13. Enforcement-mechanism taxonomy (synthesis)

| Mechanism | Exemplar | How it works | Failure mode |
|---|---|---|---|
| Session-start context injection | superpowers hook | Inject meta-skill + mandatory-invocation language every session/compact | Token cost; still persuasion |
| Iron Laws + rationalization tables + red flags | superpowers TDD/debugging/verification | Pre-rebut the agent's predicted excuses, verbatim; "letter = spirit" clause | Arms race; needs pressure-test upkeep |
| Deterministic tool-call blocking | tdd-guard/Probity | PreToolUse hook + validator model rejects non-TDD edits | Latency, setup, validator errors |
| Evidence protocols | ECC git checkpoints; superpowers verification gate | Claims require reachable commits / fresh command output | Agents can still narrate around soft variants |
| Workflow stage-gating | compound-engineering, superpowers plan→execute | Next stage requires artifact from previous | Rubber-stamp artifacts |
| Always-on ruleset + audit commands | ponytail | Persona + ladder injected; /review //audit passes after | No hard stop mid-generation |
| Learned instincts + hookify | ECC | Failures observed → hooks/instincts generated with confidence scores | Cold start; drift |
| Skill-dev-as-TDD (pressure testing) | superpowers writing-skills | Baseline subagent violation → write skill → verify compliance under combined pressures | Expensive; manual |

## 14. Measurably effective vs decorative

**Measured / evidenced:**
- **ponytail**: the only collection with a published controlled benchmark (−54% LOC, −22% tokens, −20% cost, −27% time, n=4×12 tasks, corrected methodology) — and it self-reports its earlier inflated number as an artifact.
- **SkillJuror** ([arxiv 2606.11543](https://arxiv.org/pdf/2606.11543)): identical skill content restructured across checklists / flowcharts / rationalization tables / frontmatter / progressive disclosure produces **substantial compliance differences**; structures emphasizing logical flow and explicit dependencies win. Empirical confirmation that superpowers-style organization is functional, not cosmetic.
- **superpowers' description-trap finding**: workflow-summarizing descriptions measurably cause agents to skip the skill body — the most actionable single authoring rule found in this research.
- **tdd-guard**: deterministic blocking works by construction (the edit physically doesn't land) — trade is latency/setup.
- **Claude Skills 2.0** (2026): built-in evals + blind A/B judging of skill variants ([MindStudio guide](https://www.mindstudio.ai/blog/claude-code-skills-2-evaluation-ab-testing), [Medium](https://medium.com/@muhammadwaheedairi/claude-skills-just-got-a-quality-control-system-heres-why-it-changes-everything-d8120e2c63cf)); ecosystem estimate that of ~500k circulating skills, **only ~5% are highly effective**.
- **Testing methodology for skill authors**: [How to tell if your Claude skills are any good](https://www.whytryai.com/p/how-to-test-claude-skills).

**Largely decorative (common patterns):**
- Restating textbook principles (SOLID/KISS/DRY) without triggering conditions, counter-rationalization, or verification hooks — the bulk of the ~95%.
- Coverage-percentage mandates with no evidence protocol (agent asserts "80%+" without measurement).
- Descriptions that summarize the workflow (actively harmful per superpowers' testing).
- Giant always-on rule files that blow the token budget the skill was supposed to protect (superpowers v6.1.0 release notes show even the best framework has to keep re-trimming).

**Design consensus across the effective tier:** (1) trigger-only descriptions; (2) progressive disclosure with <500-line bodies; (3) explicit Iron-Law-style absolutes for discipline skills, soft guidance only for taste; (4) pre-rebutted rationalizations captured from real baseline failures; (5) verification = fresh command output or committed artifacts, never assertion; (6) where stakes justify cost, back prose with deterministic hooks.
