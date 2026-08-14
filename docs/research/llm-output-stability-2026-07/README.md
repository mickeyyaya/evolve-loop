# How to Make LLM Deliverables Stable — Literature vs. Our Architecture (2026-07-30)

Console research pass: what the field says about getting reliable output out of
LLMs (architecture, prompts, settings, tooling), checked against this repo's
own evidence from batches 17-20 and this session's six console fixes.

## 0. The headline, and why it matters here

> **65% of enterprise AI failures trace to HARNESS defects, not model defects**,
> and "optimizing the model without stabilizing the harness yields diminishing
> returns." — [harness-engineering survey](https://www.augmentcode.com/guides/harness-engineering-ai-coding-agents)

This session is a controlled case study confirming it. Every defect I fixed
today was a **harness** defect, not a model-quality defect:

| Fix | Class |
|---|---|
| verify-read racing the agent's final write (#378) | harness I/O |
| FAIL accounting unreachable in fleet mode (#373) | harness wiring |
| abnormal-exit reasons content-free (#376) | harness diagnosability |
| profiles pin lagging the live reroute (#375) | harness config |
| leaked 8-core load harness | harness hygiene |
| `-predicates` flag silently dropped by stdlib parse | harness CLI |

The one apparent model failure (agy ignoring the deliverable contract 7/7) was
*also* half-harness: corrections re-dispatched to the same CLI forever because
`cli_fallback` fires only on infra exit codes. **Corollary for this project:
the standing "pipeline fixes console-first at highest reasoning" rule is not
process overhead — it is where the reliability actually lives.**

## 1. Settings: what is and isn't achievable

**Temperature 0 is NOT deterministic.** Non-determinism at temp 0 is dominated
by **batch-size-dependent reduction kernels** (normalization, matmul, attention)
— the same prompt under different dynamic batch sizes takes different reduction
trees. Bit-identical output is achievable only with batch-invariant kernels, at
~34-61% throughput cost ([Thinking Machines, Sept 2025](https://sulbhajain.medium.com/defeating-nondeterminism-in-llm-inference-thinking-machines-2339599e4156);
[batch-composition accuracy swings up to 9%](https://standardapplied.com/field-intelligence/non-deterministic-llm/)).

**Consequence for us:** never build a mechanism that depends on output
*identity*. Depend on **contract satisfaction**. Our architecture already does —
and this reframes the fingerprint-normalization work (#368/#370/#376) as the
*correct response to a fundamental property*, not a workaround for sloppy
agents. Two distinct aborts genuinely cannot be expected to render identically.

## 2. Architecture: what the field prescribes, and our score

| Practice | Field | Us | Gap |
|---|---|---|---|
| Deterministic outer harness (linters, CI gates) outranks probabilistic judgment | required | **EGPS/build-floor/apicover gates outrank narrative** | none — validated |
| Independence-based verification (test-writer blind to implementation, separate agents/queues) | required | **TDD-engineer (Opus) authors predicates before builder (Sonnet)**; adversarial auditor separate | none — validated |
| Structured output via **constrained decoding** (FSM token masking; 100% schema-valid; [XGrammar <40µs/token](https://arxiv.org/pdf/2501.10868)) | 2026 production standard | **prompt-instructed format + post-hoc validation + correction re-dispatch** | **REAL GAP** — see §4 |
| Compound-reliability math (0.99^10 ≈ 90%) → per-step gates | required | per-phase gates + single-writer triage | none |
| Judge calibration against ground truth ([judges have low error-recall, are overconfident](https://arxiv.org/pdf/2606.10315)) | required | verdict-conflict records both readings — **but never mined** | queued (auditor-calibration-report) |
| Context budgeting by **fill %**, compact past ~60%, cap effective window | required | no cap, no fill budget, full instruction stack ~24×/cycle | **REAL GAP** — see §3 |

## 3. Context engineering is a QUALITY lever, not just a cost lever

Across 18 frontier models, accuracy drops **30-50% well before the documented
context limit**; degradation modes are poisoning (errors replicate step to
step), distraction (model leans on accumulated history instead of re-planning),
and confusion (irrelevant tokens dilute).
2026 defaults: **budget by fill percentage, compact proactively past ~60%, cap
the effective window below the advertised one** ([context-rot playbook](https://www.digitalapplied.com/blog/context-engineering-agent-reliability-playbook-2026),
[Redis](https://redis.io/blog/context-rot/), [long-horizon diagnosis](https://arxiv.org/pdf/2606.29718)).

**This reframes our whole token-optimization queue.** `test-amplification-context-scope`
(0.89), `tokenopt-role-scoped-instruction-digests` (0.88), `tokenopt-handoff-digests`
(0.87) and `codegraph-blast-radius` (0.85) were queued as *cost* items. They are
**quality** items: every one of them removes low-signal tokens from a phase's
window. Distraction is the likeliest explanation for a real pattern we observe —
late-phase agents (audit, retro) reasoning from accumulated history rather than
re-deriving from the diff.

Cycle 1460 therefore keeps role-scoped digests evidence-gated: [LLMLingua](https://aclanthology.org/2023.emnlp-main.825/)
targets semantic integrity during compression, while a [large-scale prompt-compression
study](https://arxiv.org/abs/2604.02985) finds that benefits depend on measured
quality and operating conditions. The runner records size/parity in shadow mode
and preserves the full prompt for unsafe projections before any default flip.

Missing entirely: **fill-percentage telemetry and an effective-window cap.** We
measure tokens spent; we do not measure how *full* each phase's window was when
it produced its deliverable. Without that we cannot correlate quality with fill,
which is the one experiment that would settle how much the digest work buys.

## 4. Prompts: where our format-adherence failures actually come from

The agy 7/7 contract failure is the textbook case the constrained-decoding
literature exists to solve: we ask for a format in prose and validate after the
fact. We drive **interactive CLI REPLs** (claude/codex/agy under tmux), so
FSM-level constrained decoding is not available on that channel — an
architectural constraint worth naming rather than wishing away. Three
compensating controls, in order of leverage:

1. **Escalate the CLI on repeated contract blocks** (already queued,
   `contract-block-cli-escalation` 0.9). Empirically the only thing that worked:
   agy went 0-for-7 on corrections; claude complied. **Format adherence is a
   model property past some point, not a prompt property.**
2. **Emit the machine-readable verdict as a TOOL CALL where the CLI supports
   one** — the closest available equivalent to constrained decoding, and it
   removes the last-line-of-prose fragility that also caused the read race.
3. **Put hard contract requirements at the generation point.** Anthropic's own
   guidance: Claude follows instructions in the **user turn** better than the
   system message, and XML-tagged sections parse unambiguously
   ([platform docs](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices)).
   Our required-sections + sentinel template currently live in the persona body
   (system-ish, far from generation); the correction prompt that *does* work puts
   them in the turn tail. That asymmetry is a free experiment.

## 5. Self-consistency: use it on DECISIONS, never on the builder

Majority voting over N samples is "the best dollar-per-accuracy in the ladder for
reasoning tasks," with negligible marginal gain past N≈20 — but it **reduces
variance, not bias**: when the model is systematically wrong, all samples agree
and voting cannot help ([survey](https://arxiv.org/pdf/2406.06608),
[self-consistency estimation](https://arxiv.org/pdf/2509.19489)).

Applied to us: the builder's failures are **bias** (it misunderstands the task),
so N>1 there is pure cost. The high-leverage targets are cheap, single-shot
**decision** points where one bad sample wastes an entire cycle:

- **triage `top_n` selection** — a bad menu burns a whole cycle downstream
- **verdict-conflict adjudication** — exactly the cases where narrative and gate
  disagree today
- **spec reachability** (does this acceptance criterion admit a passing state?)

N=3 majority on those three decisions only, budget-bounded.

## 6. Evidence from this session's own subagents

Two implementer subagents produced high-quality, tested work from prompts that
carried: a precise objective, a TDD-first mandate, **explicit verification
commands with the expected report shape** ("report exact counts"), scope
boundaries ("do NOT commit"), and a required output format. That is the
prompt-engineering pattern, confirmed first-hand.

Both nevertheless **under-optimized integration**: one left a struct field with
no consumer (dead plumbing); the other built a lint with no caller. They
satisfied the stated contract exactly and did not infer the unstated one.

> **Lesson: subagents optimize the stated objective. Integration must be an
> explicit acceptance criterion, not an assumption.** This repo already has a
> "wiring proof mandatory" policy — but it lives in operator lore and my ad-hoc
> prompts, so agents don't inherit it. It belongs in the builder persona.

Also confirmed: the **budget-capped, scope-contracted** `diff-review` skill
(≤20 tool calls, staged diff only) caught three genuine BLOCKs today at roughly
a third of the tokens unbounded review previously used. Constraining a
reviewer's scope contract *raises* its quality.

## 7. Queued from this review

| Item | Weight | Lever |
|---|---|---|
| `contract-requirements-at-generation-point` | 0.90 | format adherence (§4.3) |
| `builder-persona-requires-caller-proof` | 0.90 | integration completeness (§6) |
| `context-fill-telemetry-and-cap` | 0.89 | context rot (§3) |
| `self-consistency-on-decision-phases` | 0.87 | decision variance (§5) |
| `verdict-sentinel-as-tool-call` | 0.86 | format adherence (§4.2) |
| annotations on the tokenopt trio + `contract-block-cli-escalation` | existing | reframed as quality |

## Sources

- Harness: https://www.augmentcode.com/guides/harness-engineering-ai-coding-agents · https://arxiv.org/pdf/2605.25665 · https://arxiv.org/pdf/2604.17025 · https://martinfowler.com/articles/harness-engineering.html
- Determinism: https://sulbhajain.medium.com/defeating-nondeterminism-in-llm-inference-thinking-machines-2339599e4156 · https://standardapplied.com/field-intelligence/non-deterministic-llm/ · https://arxiv.org/pdf/2407.10457 · https://arxiv.org/pdf/2601.06118
- Structured output: https://arxiv.org/pdf/2501.10868 · https://arxiv.org/pdf/2603.03305 · https://arxiv.org/pdf/2605.02363
- Context: https://www.digitalapplied.com/blog/context-engineering-agent-reliability-playbook-2026 · https://redis.io/blog/context-rot/ · https://arxiv.org/pdf/2606.29718 · https://arxiv.org/pdf/2603.09619
- Prompting: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices
- Self-consistency: https://arxiv.org/pdf/2406.06608 · https://arxiv.org/pdf/2509.19489
- Judges: https://arxiv.org/pdf/2606.10315 · https://arxiv.org/pdf/2512.22245 · https://arxiv.org/pdf/2507.08794
