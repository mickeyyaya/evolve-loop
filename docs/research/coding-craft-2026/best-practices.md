# Coding Discipline for the Agentic Era — Research Findings (2025–2026)

Researched 2026-07-07. Scope: latest authoritative guidance worth encoding into an LLM coding skill; Go-primary but language-general. Each entry: **Source + date · Actionable rule · Evidence quality** (measured = quantitative data; opinion = expert/practitioner consensus; vendor = vendor guidance informed by internal usage).

---

## Quick index — top actionable rules

1. Every task gets a **machine-runnable check** (test/build/screenshot-diff) the agent iterates against; evidence over assertion. (Anthropic, vendor)
2. **Red-first with separated contexts**: test-writer and implementer in different agent contexts; commit the failing test; forbid test edits during green phase. (arXiv + OpenAI, measured/vendor)
3. Judge tests by **mutation kill rate, not coverage** — 85% coverage can hide a 57% kill rate. (Meta/Atlassian/Augment, measured)
4. **Anti-mock rule in agent config**: agents add mocks in 36% of commits vs 26% for humans; mock only process boundaries. (arXiv 2602.00409, measured)
5. **Bug fix = regression twin + preservation tests**: failing bug-condition test on unfixed code, passing preservation baseline, both green after; never ship if previously-passing tests fail. (Kiro, measured+practitioner)
6. **Small functions survived scrutiny** (median CC 1–2, 3–10 LOC at Apache/Google/Microsoft); exact-number caps did not — treat length as a smell threshold, not a hard rule. (arXiv 2507.19721, measured)
7. **Duplication is the #1 measured AI-code pathology** (+81% block duplication vs 2023; refactoring moves down to 3.8% of changed lines) — make dedup/refactor passes explicit agent tasks. (GitClear, measured)
8. **Reviewability is the metric**: diff budget (≤500–800 changed lines), staged landing, convention matching, self-review before handoff. (OpenAI codex AGENTS.md + survey data, vendor/measured)
9. **No single-implementation interfaces in Go**; define interfaces at the consumer; rule-of-three before abstracting; LLMs over-engineer ~2× (guard clauses, indirection). (Go community + Kiro-cited research, opinion/measured)
10. **Comments = why, never narration**; strip "AI slop" comments that restate code; keep intent/constraint comments that help future readers and agents. (practitioner consensus, opinion)

---

## Area 1 — TDD in the agentic era

### 1.1 TDAD: Test-Driven Agentic Development
- **Source:** [TDAD: Test-Driven Agentic Development — Reducing Code Regressions via Graph-Based Impact Analysis](https://arxiv.org/html/2603.17973v1), arXiv, Mar 2026.
- **Finding:** Behavioral specs are compiled into executable test suites by one agent; a second agent iterates the implementation until tests pass; **mutation testing validates the tests themselves**. The human role is confined to specification authorship and mutation-score review. Graph-based impact analysis limits which code the agent may touch, reducing regressions.
- **Rule:** Split spec→test and test→code between agents; gate the test suite itself with mutation score; constrain the implementer's blast radius via impact analysis.
- **Evidence:** Measured (research prototype with evaluation).

### 1.2 Context separation is what makes LLM TDD real
- **Source:** [A Claude Code TDD Skill: Forcing Red-Green-Refactor Discipline](https://alexop.dev/posts/custom-tdd-workflow-claude-code-vue/), alexop.dev, 2025; [TDFlow: Agentic Workflows for Test Driven Development](https://arxiv.org/pdf/2510.23761), arXiv, Oct 2025; [TDD Governance for Multi-Agent Code Generation via Prompt Engineering](https://arxiv.org/abs/2604.26615), arXiv, Apr 2026.
- **Finding:** When test-writing and implementation share one context window, "test writer analysis bleeds into implementation thinking" — the LLM writes tests that mirror the implementation it already intends. Genuine test-first behavior requires each phase agent to start with exactly the context it needs. TDD-governance work formalizes red-green-refactor as workflow-level gates (planning/generation/repair/validation stages) rather than prompt suggestions.
- **Rule:** Run RED phase in a separate agent/context from GREEN; verify the test actually fails before implementation; enforce phases as workflow gates, not prose instructions.
- **Evidence:** Opinion (practitioner) + measured (arXiv workflow papers).

### 1.3 OpenAI's test-first checkpoint pattern
- **Source:** [Codex Best Practices](https://developers.openai.com/codex/learn/best-practices), OpenAI Developers, 2025 (living doc).
- **Finding:** Recommended flow: write tests first, **confirm they all fail**, commit the failing tests as a checkpoint, then instruct the agent to implement until green **with an explicit instruction not to modify the tests**. Task framing = Goal / Context / Constraints / Done-criteria.
- **Rule:** Commit the red test as an immutable checkpoint; the implementing agent may not edit tests; "done" is defined by the pre-committed check.
- **Evidence:** Vendor guidance.

### 1.4 Mutation testing at scale: quality over coverage
- **Source:** [LLMs Are the Key to Mutation Testing and Better Compliance](https://engineering.fb.com/2025/09/30/security/llms-are-the-key-to-mutation-testing-and-better-compliance/), Meta Engineering, Sep 30 2025 ([InfoQ coverage](https://www.infoq.com/news/2026/01/meta-llm-mutation-testing/), Jan 2026); [Automating Mutation Coverage with AI](https://www.atlassian.com/blog/development/automating-mutation-coverage-with-ai), Atlassian, 2025; [Mutation Testing for AI-Generated Code](https://www.augmentcode.com/guides/mutation-testing-ai-generated-code), Augment Code, 2025.
- **Finding:** Meta's ACH generates fault-class-targeted mutants + tests guaranteed to kill them; engineers **accepted 73% of generated tests** (36% privacy-relevant); an LLM equivalence detector filters unkillable mutants before human review. Augment's practical guide documents the core gap: AI can produce a test suite with **85% line coverage but a 57% mutation kill rate**.
- **Rule:** For critical packages, sample mutation kill rate as the test-quality gate; never accept coverage % alone as evidence tests work. Target mutants at fault classes you care about (privacy, security, off-by-one), not random operators.
- **Evidence:** Measured (production deployment at Meta scale).

### 1.5 Empirical character of AI-written tests
- **Source:** [Testing with AI Agents: An Empirical Study of Test Generation Frequency, Quality, and Coverage](https://arxiv.org/html/2603.13724), arXiv, Mar 2026; [ASTER (IBM Research)](https://research.ibm.com/blog/aster-llm-unit-testing), 2025; [Evaluating LLM-Based Test Generation Under Software Evolution](https://arxiv.org/html/2603.23443v1), arXiv, Mar 2026.
- **Finding:** AI-generated tests have longer code and **higher assertion density** than human tests, with lower cyclomatic complexity (linear logic); coverage gains comparable to human tests but **scope is more localized** in complex contributions. Known LLM test anti-patterns: non-compiling tests, redundant tests within a suite, trivial/ineffective assertions, tests lacking "naturalness"/readability, and suites that decay under software evolution.
- **Rule:** Review AI tests for (a) at least one behavior-bearing assertion per test (reject assertion-free/smoke-only tests), (b) redundancy against the existing suite, (c) whether they test the contract vs. mirroring the implementation.
- **Evidence:** Measured (empirical studies).

### 1.6 Over-mocking is an agent-specific anti-pattern
- **Source:** [Are Coding Agents Generating Over-Mocked Tests? An Empirical Study](https://arxiv.org/abs/2602.00409), arXiv, submitted Jan 30 2026.
- **Finding:** Analysis of **1.2M commits across 2,168 TS/JS/Python repos (2025)**: 36% of agent commits touching tests add mocks vs 26% for non-agents; 23% of agent commits add/change test files vs 13% for humans; 68% of repos with agent test activity also show agent mock activity. Mocked tests are easier to auto-generate but validate less real behavior; authors explicitly recommend **mocking guidance in agent configuration files**.
- **Rule:** Put a mocking policy in CLAUDE.md/AGENTS.md: mock only process boundaries (network, clock, filesystem where needed); never mock the unit under test or in-process collaborators you own; prefer fakes/real implementations; a test that only asserts mock interactions is a defect.
- **Evidence:** Measured (large-scale empirical study).

### 1.7 Property-based testing works with agents — grounded in docs
- **Source:** [Finding bugs with Claude and property-based testing](https://red.anthropic.com/2026/property-based-testing/), Anthropic, Jan 14 2026; [Understanding the Characteristics of LLM-Generated Property-Based Tests](https://2025.aiwareconf.org/details/aiware-2025-papers/25/), AIware 2025; [PBT-Bench](https://arxiv.org/pdf/2605.15229), arXiv 2026; [From Prompts to Properties](https://dl.acm.org/doi/10.1145/3696630.3728702), FSE 2025.
- **Finding:** Anthropic's PBT agent (Hypothesis-based) tested 100+ Python packages: 984 bug reports, **56% valid; 86% of top-ranked reports valid**; patches merged in NumPy/SciPy/Pandas. Three principles: (1) ground properties in explicit documentation/usage, not assumptions; (2) self-reflection loop to check a failure is a genuine bug; (3) rank findings by severity/confidence. AIware 2025: combining PBT with example-based tests raised bug detection to **81.25%** vs either alone.
- **Rule:** For pure functions, parsers, encoders, and anything with round-trip/invariant structure (Go: use `testing/quick` or `rapid` + native fuzzing), add property tests alongside example tests; derive properties from documented contracts only; treat property failures as hypotheses to verify, not auto-filed bugs.
- **Evidence:** Measured (vendor research with hard numbers + peer-reviewed studies).

### 1.8 Regression-twin discipline: bug condition + preservation
- **Source:** [The bug fix paradox: why AI agents keep breaking working code](https://kiro.dev/blog/bug-fix-paradox/), Kiro (AWS), Feb 19 2026; [GoClaw: AI Agent Fix Bugs](https://goclaw.sh/blog/ai-agent-fix-bugs), 2025–26.
- **Finding:** Agents fixing bugs over-engineer: cited research shows agents are **~2× as likely as humans to introduce unnecessary guard clauses**, plus drive-by refactors and defensive null checks. Kiro's property-aware workflow: write **bug-condition tests** (fail on unfixed code) and **preservation tests** (pass on unfixed code) *before* the fix; after the fix both must be green. Practitioner rule: the agent may not open a PR if any previously-passing test now fails.
- **Rule:** Every bug fix ships a regression twin (test that failed pre-fix) plus a preservation check; the diff may not touch code outside the bug condition's blast radius.
- **Evidence:** Measured (cited study) + practitioner methodology.

---

## Area 2 — Clean code 2026: what survived empirical scrutiny

### 2.1 Small functions: confirmed in practice; magic numbers: not
- **Source:** [Clean Code In Practice: Challenges and Opportunities](https://arxiv.org/html/2507.19721v1), arXiv, Jul 2025 — 150 OSS projects from Apache/Google/Microsoft across C/C++/Java/JS/Python.
- **Finding:** Industry code overwhelmingly satisfies "small functions": **median cyclomatic complexity 1–2, median function length 3–10 LOC**; 7–17% outliers depending on language/org. Files mostly ≤600 LOC (Q3); lines ≤~100 chars. But **naming violations are widespread and uneven** (irregular function names 0.1–21.8%; Microsoft Java class names 17.7% irregular). Authors warn against mandatory one-size-fits-all enforcement and tool false positives.
- **Rule:** Keep the "small function/file" discipline (it's what elite codebases actually do) but enforce as reviewable smell thresholds (e.g., flag >50-line functions, >800-line files) rather than hard gates; enforce naming with linters since it's the most-violated rule in practice.
- **Evidence:** Measured (large multi-org corpus study).

### 2.2 There is no empirically "ideal" function length
- **Source:** [How Developers Extract Functions: An Experiment](https://arxiv.org/pdf/2209.01098) (updated through 2025 revisions); Fenton & Neil's critique of the "Goldilocks conjecture" (cited in 2025 literature reviews).
- **Finding:** Very little empirical support for a specific optimal length; the one classic study (COBOL maintenance) found ~44 statements optimal — contradicting the "≤5 lines" Clean Code prescription. Extraction decisions in practice are driven by cohesion/naming ability, not line counts.
- **Rule:** Extract when a fragment has a name and a single reason to change, not to hit a number; do not let an LLM shred logic into 3-line fragments to satisfy a length rule (that raises indirection cost, which is what harms comprehension).
- **Evidence:** Measured but sparse/contested — honest status: length rules are convention, not science.

### 2.3 The measured AI-code pathologies: duplication up, refactoring collapsed
- **Source:** [GitClear AI Copilot Code Quality 2025](https://www.gitclear.com/ai_assistant_code_quality_2025_research) (211M changed lines; Feb 2025) and [The Maintainability Gap: 2026 AI Code Quality Research](https://www.gitclear.com/the_ai_code_quality_maintainability_gap) (2026).
- **Finding:** 2024: duplicated code blocks up **8×**; copy/pasted lines 8.3%→12.3% (2021→2024). 2026 YTD: block duplication **73.0 per million changed lines, +81% vs 2023** (highest on record); moved/refactored lines down to **3.8%** of changed lines (was ~25% in 2021, −70%); cross-file reuse −35%; two-week churn +15%; error-masking constructs +47%.
- **Rule:** For agent-written code: (1) require a search-for-existing-implementation step before writing new helpers; (2) schedule explicit consolidation/refactor cycles — agents won't refactor spontaneously; (3) track churn and duplication as first-class repo health metrics.
- **Evidence:** Measured (largest longitudinal dataset available), though GitClear sells the measuring tool — mild COI.

### 2.4 Reviewability is the 2026 quality metric
- **Source:** [CodeRabbit: 2025 was the year of AI speed, 2026 will be the year of AI quality](https://www.coderabbit.ai/blog/2025-was-the-year-of-ai-speed-2026-will-be-the-year-of-ai-quality) (Dec 2025); [Second Talent metrics roundup](https://www.secondtalent.com/resources/ai-generated-code-quality-metrics-and-statistics-for-2026/) (2026) aggregating GitClear/Veracode/Stack Overflow 2025; [metacto: Code Review Standards for AI-Generated Code](https://www.metacto.com/blogs/establishing-code-review-standards-for-ai-generated-code) (2026).
- **Finding:** AI PRs contain **~1.7× more defects**; logic/correctness errors 1.75× higher; **45% of AI-generated code samples contain an OWASP Top-10 flaw** (Veracode 2025); Stack Overflow 2025: only 3% of devs highly trust AI code, **71% won't merge without manual review**, 66% report fixing "almost right" code; senior engineers report **20–35% more review time**. The bottleneck has moved from writing to reviewing — so the generator must optimize for the reviewer.
- **Rule:** Optimize agent output for review cost: minimal diff, one concern per PR, no drive-by changes, self-review pass before handoff, attach evidence (test output, commands run). "Almost right" is the dominant failure mode — treat plausible-looking code as unverified by default.
- **Evidence:** Measured (multiple independent datasets: Veracode, Stack Overflow survey n>49k, GitClear).

### 2.5 DORA 2025: AI amplifies your existing discipline — including the bad
- **Source:** [DORA State of AI-assisted Software Development 2025](https://dora.dev/dora-report-2025/), Google Cloud, Sep 2025; [Google Cloud: TDD and AI](https://cloud.google.com/discover/how-test-driven-development-amplifies-ai-success); [InfoQ summary](https://www.infoq.com/news/2026/03/ai-dora-report/), Mar 2026.
- **Finding:** ~5,000-dev survey: 90% AI adoption (+14pp YoY); trust remains low (only 24% report high trust). AI adoption correlates with **higher perceived code quality (59% positive) AND higher delivery instability** — AI is an amplifier, not a substitute: "mature version control workflows, disciplined code review, and consistent development standards form the backbone... rather than replacing these practices, AI depends on them." Google explicitly positions TDD as the practice that amplifies AI success (fast, trustworthy feedback loops).
- **Rule:** Do not relax gates because AI is fast; strengthen them: every AI change flows through version control + automated tests + human review; small-batch delivery matters more, not less, with AI.
- **Evidence:** Measured (large annual survey; correlational).

### 2.6 Naming / immutability / error-handling consensus (stable, language-tuned)
- **Source:** Cross-cutting: Clean Code In Practice (2025), [Cut the Clutter (dev.to, 2025)](https://dev.to/maxed/cut-the-clutter-why-you-should-ditch-useless-code-comments-5dcn), Go review comments (living doc), error-handling sections of the AI-code-review literature above.
- **Finding:** The rules that survived: intention-revealing names (most violated in practice — see 2.1), explicit error handling at boundaries (error-masking constructs are up **47%** in AI code per GitClear — swallowed errors are a top AI defect class), immutability-by-default for shared data (unchanged consensus; in Go: return copies, avoid shared mutable package state). What did NOT survive: comment-density targets and "self-documenting code needs no comments" absolutism — see 4.4.
- **Rule:** Encode as: never swallow errors (Go: never `_ =` an error without a comment naming why; wrap with `%w` and context); prefer value semantics/copies for shared data; names must state intent (linter-enforced casing + reviewer-enforced semantics).
- **Evidence:** Measured for error-masking trend; opinion/consensus for the rest.

---

## Area 3 — Design patterns: what matters, what LLMs misuse

### 3.1 Go: single-implementation interfaces are the canonical smell
- **Source:** [Two common Go interface misuses](https://konradreiche.com/blog/two-common-go-interface-misuses/), Konrad Reiche; [Interfaces Are Not Meant for That](https://preslav.me/2023/12/15/golang-interfaces-are-not-meant-for-that/), Preslav Rachev, Dec 2023; [You Are Misusing Interfaces in Go](https://medium.com/goturkiye/you-are-misusing-interfaces-in-go-architecture-smells-wrong-abstractions-da0270192808), Emre Savcı.
- **Finding:** The most frequent Go smell is prematurely derived interfaces imported from Java-style OO habit: interfaces created with a single implementation, interfaces created **solely to enable mocking** ("weakens the expressiveness of your types and reduces readability long-term"), and interfaces defined next to the producer instead of the consumer. LLMs trained on Java/TS corpora reproduce exactly this.
- **Rule (Go-specific, encode verbatim):** Define interfaces at the point of use (consumer package), only when ≥2 real implementations exist or the consumer genuinely needs substitution; accept interfaces, return concrete structs; do not create an interface to mock — restructure for a fake or test the real thing.
- **Evidence:** Opinion — but strong, convergent expert consensus aligned with official Go proverbs/CodeReviewComments.

### 3.2 LLM over-engineering: the documented anti-pattern set
- **Source:** [KISS Sorcar](https://arxiv.org/pdf/2604.23822), arXiv 2026; Kiro bug-fix paradox (Feb 2026, see 1.8); the `ai-generated-code-review` practitioner literature 2025–26.
- **Finding:** LLMs systematically "over-engineer solutions, introducing unnecessary abstractions, helper classes, and levels of indirection," plus pointless intermediate variables (assign-then-return), unnecessary guard clauses (~2× human rate), and defensive code for impossible states. Anthropic's own best-practices doc warns that reviewer agents prompted to find gaps will produce findings even for sound code, and chasing them "leads to over-engineering: extra abstraction layers, defensive code, and tests for cases that can't happen."
- **Rule:** Post-generation simplification pass is mandatory (inline single-use indirection, delete dead parameters/defensive branches); reviewer findings are filtered to correctness/requirements only; never accept an abstraction introduced for a hypothetical future caller.
- **Evidence:** Measured (guard-clause rate) + vendor + convergent practitioner reports.

### 3.3 When NOT to abstract: rule-of-three, YAGNI, post-architecture
- **Source:** [Post-Architecture: Premature Abstraction Is the Root of All Evil](https://arendjr.nl/blog/2024/07/post-architecture-premature-abstraction-is-the-root-of-all-evil/), Arend van Beelen, Jul 2024 (widely cited through 2025-26); GitClear duplication data (2.3) as the counter-pressure.
- **Finding:** Current consensus threads the needle: premature abstraction is worse than duplication ("prefer duplication over the wrong abstraction"), but the GitClear data shows AI has swung teams to the opposite failure (mass duplication, zero consolidation). The synthesis practitioners converge on: duplicate freely up to ~3 occurrences (rule of three), then consolidate deliberately — and treat consolidation as an explicit scheduled task because agents won't do it emergently.
- **Rule:** Rule of three before any new abstraction; three similar lines beat a premature helper; but track duplication and run consolidation passes — YAGNI is not a license for copy-paste sprawl.
- **Evidence:** Opinion (design essays) + measured counter-signal (GitClear).

### 3.4 Patterns that earn their keep in agent-era codebases: DI, Strategy, ports/adapters
- **Source:** [Backend Coding Rules for AI Coding Agents: DDD and Hexagonal Architecture](https://medium.com/@bardia.khosravi/backend-coding-rules-for-ai-coding-agents-ddd-and-hexagonal-architecture-ecafe91c753f), 2025; [Hexagonal Architecture: Isolate AI Logic Effectively](https://sparkco.ai/blog/hexagonal-architecture-isolate-ai-logic-effectively), 2025; [HexDDD AGENTS.md](https://github.com/GodSpeedAI/HexDDD/blob/main/AGENTS.md).
- **Finding:** Hexagonal/ports-adapters gained a *new* justification in 2025–26: **context locality for agents**. Strict dependency direction (adapters→ports; ports live in the domain; domain depends on nothing) means an agent can work on a single well-scoped module without loading the whole codebase; non-deterministic LLM integrations get quarantined behind adapters. DI stays relevant as composition-root wiring (constructor injection), NOT as container magic. Strategy/Specification remain the sanctioned alternative to feature-flag sprawl for behavior variation.
- **Rule:** Enforce dependency direction as an architecture test (Go: import-boundary checks); wire dependencies at the composition root via constructors; vary behavior via Strategy objects selected by config/policy, not boolean flags; put external/non-deterministic calls (LLMs, network) behind an adapter with a fake for tests.
- **Evidence:** Opinion/practitioner — coherent with measured context-window constraints of agents.

---

## Area 4 — AI-coding-specific published guidance

### 4.1 Anthropic: give the agent a check; explore→plan→code; evidence over assertion
- **Source:** [Claude Code Best Practices](https://code.claude.com/docs/en/best-practices), Anthropic, living doc (2025–26); [Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents), Anthropic Engineering 2025.
- **Finding:** Core doctrine: (1) "Give Claude a check it can run" — a pass/fail signal (tests, build exit code, screenshot diff) closes the loop; without it, "looks done" is the only signal. (2) Explore→plan→implement→commit for multi-file/uncertain work; skip planning "if you could describe the diff in one sentence." (3) Bug prompts should say "write a failing test that reproduces the issue, then fix it" and "address the root cause, don't suppress the error." (4) Have the agent show evidence (test output, commands run) rather than asserting success. (5) Adversarial review in a fresh context ("nothing outside the task's scope changed" is an explicit check), with the caveat to ignore non-correctness findings to avoid over-engineering. (6) CLAUDE.md discipline: include only what can't be inferred from code; "for each line ask: would removing this cause mistakes? If not, cut it"; convert repeated behavioral rules into deterministic hooks.
- **Rule set to encode:** machine-verifiable done-criteria per task; failing-repro-first for bugs; root-cause not suppression; evidence-bearing completion reports; fresh-context diff review scoped to correctness + scope adherence.
- **Evidence:** Vendor guidance distilled from internal usage at scale.

### 4.2 OpenAI: AGENTS.md contract + hard diff budgets
- **Source:** [Custom instructions with AGENTS.md](https://developers.openai.com/codex/guides/agents-md) and [Codex Best Practices](https://developers.openai.com/codex/learn/best-practices), OpenAI, 2025–26; [openai/codex AGENTS.md](https://github.com/openai/codex/blob/main/AGENTS.md).
- **Finding:** AGENTS.md should contain "information the agent cannot reliably infer from the repository but must follow": commands first (setup/test/lint), conventions, **do-not rules**, and "what done means and how to verify." Add rules only after observing repeated mistakes (rules are patches for observed failures, not aspirations). OpenAI's own repo caps agent changes: **≤800 changed lines for mechanical changes, ≤500 for complex logic**; larger work must be split into reviewable stages, landing the smallest coherent stage first.
- **Rule:** Adopt explicit diff budgets (≈500 complex / 800 mechanical changed lines) with a split-into-stages requirement; grow instruction files empirically from observed failures, lead with commands.
- **Evidence:** Vendor guidance; the diff cap is OpenAI's real in-repo policy for its own agent-heavy repo.

### 4.3 Scope discipline: enumerate what NOT to touch
- **Source:** [AI Agent Scope Creep: Why Agents Expand Their Own Scope](https://www.armalo.ai/learn/scope-creep-ai-agents-prevention), 2025–26; Kiro preservation-property workflow (1.8); Anthropic scope-adherence review check (4.1).
- **Finding:** Agents expand scope by default (helper refactors, "while I'm here" cleanups, extra tests for passing cases). Effective prevention is contract-shaped: enumerate in-scope files/behaviors AND out-of-scope items, verify post-hoc that "nothing outside the task's scope changed," and trace each sub-change to a requirement. Convention matching is part of scope: match the codebase's existing case/idioms/organization even when the model "knows better"; style changes go in separate refactor commits.
- **Rule:** Task specs name in-scope files + explicit non-goals; final diff review includes a scope-adherence check; convention deviations and refactors are separate commits, never bundled with feature/fix work.
- **Evidence:** Opinion/practitioner + vendor alignment (Anthropic, Kiro, OpenAI all independently converge).

### 4.4 Comment policy: why-comments in, narration out
- **Source:** [The Case for Comment-Driven Development](https://www.usetusk.ai/resources/the-case-for-comment-driven-development), Tusk, 2025; [Cut the Clutter](https://dev.to/maxed/cut-the-clutter-why-you-should-ditch-useless-code-comments-5dcn), dev.to 2025; "AI code slop" reviewer prompts (2025–26); Glean [How AI assistants interpret code comments](https://www.glean.com/perspectives/how-ai-assistants-interpret-code-comments-a-practical-guide).
- **Finding:** 2025–26 consensus rejects both extremes. Banning agent comments loses value: intent comments ("why") measurably help *future agent runs* retrieve context and reduce misinterpretation. But AI defaults to narration slop (`// set user to active` above `user.status = 'active'`), which reviewers now treat as a tell of unreviewed AI output. The policy that emerged: comments must carry information not derivable from the code — constraints, invariants, links to decisions/gotchas; everything else is deleted in the simplification pass.
- **Rule:** Comments explain why/invariants/gotchas, never restate the code; the post-generation pass deletes narration comments; do NOT add change-narration comments ("// added for bug X") — that's what commit messages are for.
- **Evidence:** Opinion — but unusually convergent across practitioner + tooling vendors.

### 4.5 Spec-driven governance as the umbrella model
- **Source:** [The Productivity-Reliability Paradox: Specification-Driven Governance for AI-Augmented Software Development](https://arxiv.org/pdf/2605.01160), arXiv, May 2026; [Evaluation-Driven Development of LLM Agents](https://arxiv.org/pdf/2411.13768).
- **Finding:** The research framing for 2026: AI raises raw throughput while lowering per-change reliability; the resolution is governance by executable specification — human-authored specs compiled to tests/evals, machine-checkable gates at every stage, humans reviewing at the spec/gate level rather than line level. This matches what Anthropic (/goal + Stop hooks), OpenAI (done-criteria), and Google DORA (gates amplify AI) each independently ship.
- **Rule:** Every agent task carries: spec → executable acceptance check → gated pipeline → evidence-bearing report. Human attention goes to specs and gate design, not to babysitting generation.
- **Evidence:** Measured framing (arXiv) + triangulated vendor convergence.

---

## Synthesis for a Go-primary LLM coding skill

**Encode as hard rules (evidence: measured):** regression-twin + preservation tests per bug fix; mutation-sampled test quality on critical paths; anti-mock policy; duplication search-before-write + scheduled consolidation; diff budgets with staged landing; error-swallowing ban.

**Encode as strong defaults (evidence: convergent vendor/practitioner):** red-first in separate context with committed failing test; machine-verifiable done-criteria; explore→plan→code for multi-file work; scope contracts with explicit non-goals; why-only comments; consumer-side, multi-implementation-only interfaces (Go); rule-of-three; composition-root DI + Strategy over flags; ports/adapters for external and non-deterministic calls.

**Do NOT encode (evidence contradicts or absent):** hard function-length numbers (no empirical ideal — use smell thresholds); comment-density targets; coverage-percentage gates as proof of test quality; "no comments needed if code is clean" absolutism; interface-per-struct or mock-driven design.
