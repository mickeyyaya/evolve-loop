# Evolve Loop

**An autonomous pipeline that improves your codebase across many cycles — and structurally detects when the AI tries to fake the result.**

> **The pitch, in one sentence:** Any agent can write a feature overnight. Evolve Loop is the layer that decides whether that code is *safe to merge* — adversarially, structurally, and with memory that compounds across runs.

The mental model is **CI/CD for AI-written code**. You hand it a goal — "add dark mode," "harden the auth flow," "pay down concurrency debt" — and, optionally, a number of cycles; leave the count off and the advisor decides how many the work needs. It runs unattended: it finds the work, plans it, writes it, has a *different* model adversarially review it, ships only what passes deterministic checks, and turns every failure into a durable lesson the next cycle reads automatically.

It works with Claude Code, Gemini CLI, and Codex CLI — and can route a different LLM to each stage of the work.

> **Prefer to see it?** The same story — the moved bottleneck, the pillars, the self-caught incident — is laid out visually on the **[Evolve Loop landing page](https://mickeyyaya.github.io/evolve-loop/)** (flagship version: **[noir](https://mickeyyaya.github.io/evolve-loop/noir/)**).

---

## Quick Start

**Prerequisites:** one supported CLI — [Claude Code](https://docs.anthropic.com/en/docs/claude-code), [Gemini CLI](https://github.com/google-gemini/gemini-cli), or [Codex CLI](https://github.com/openai/codex) — and a repo you want to improve. The installer auto-installs the rest (`git`, `jq`, `tmux`).

**Install — one line.** Detects your OS/arch, downloads the prebuilt `evolve` binary (or builds from source as a fallback), and installs evolve for the CLI(s) you have:

```bash
curl -fsSL https://mickeyyaya.github.io/evolve-loop/install.sh | sh
```

Wary of `curl | sh`? Inspect first: `curl -fsSL https://mickeyyaya.github.io/evolve-loop/install.sh -o install.sh && less install.sh && sh install.sh`. Already in Claude Code? You can instead add the plugin directly — `/plugin marketplace add mickeyyaya/evolve-loop` then `/plugin install evo@evo`.

**Windows:** the autonomous loop's runtime is Unix-based (tmux, bash), so run it under **[WSL2](https://learn.microsoft.com/windows/wsl/install)** — install WSL, open your WSL (e.g. Ubuntu) shell, then run the same one-liner there (inside WSL, it installs exactly as on Linux). The `/evo:*` skills *also* install natively in Claude Code on Windows via the `/plugin` commands above; only the loop runtime needs WSL.

### Run — from one command to full control

Start with just a goal. Reach for a flag only when you want more control — each step below adds one, and explains what happens behind the scenes.

**1 · Just a goal** — the advisor decides everything:

```bash
/evo:loop "add dark mode"
```
> *Behind the scenes:* Scout reads your repo and the goal, then the advisor **composes the pipeline itself** — which phases to run, which LLM + model runs each, and how many cycles. It keeps going until the work backlog is drained, bounded by a safety cap. You configure nothing.

**2 · Steer the approach** with a strategy:

```bash
/evo:loop harden                              # stability + tests
/evo:loop repair "fix the auth bug"           # smallest-diff bug fix
/evo:loop innovate "explore concurrency"      # new capabilities
```
> *Behind the scenes:* a strategy (`balanced` default · `harden` · `repair` · `innovate` · `ultrathink` · `autoresearch`) shifts **what Scout looks for and how strict Audit is** — same phase spine, different posture.

**3 · Bound the cycles** when you want a hard limit:

```bash
/evo:loop --cycles 3 "add dark mode"
```
> *Behind the scenes:* `--cycles N` is a **contract** — exactly N cycles, never early-stopped. Omit it (step 1) and the advisor decides the count instead.

**4 · Control the models** with a one-time setup:

```bash
/evo:setup                          # pick a preset once
/evo:loop "harden the auth flow"
```
> *Behind the scenes:* setup writes **per-phase model routing** to `.evolve/policy.json` — e.g. Build on Codex/GPT-5.5 and Audit on Claude/Opus, deliberately **different model families so the reviewer can't rubber-stamp the builder**. Every later run uses it.

**5 · Resume** a run that was interrupted:

```bash
/evo:loop --resume
```
> *Behind the scenes:* a checkpoint (e.g. a quota wall mid-cycle) is picked up **exactly where it stopped** — the worktree and cycle state are restored, no work lost.

A hands-on walkthrough of your first cycle: [docs/getting-started/your-first-cycle.md](docs/getting-started/your-first-cycle.md).

### Setup & configuration (optional)

You can run the loop with **zero configuration** — it defaults to all-Claude models and sensible behavior. When you want control, it's one command:

```bash
/evo:setup
```

It detects which LLM CLIs you have, explains the pipeline, and offers **three presets** — **Recommended**, **Economy** (cheaper/faster models), and **Max-quality** (strongest models). You make **one choice**, and it writes the per-phase model routing for you. Re-run it anytime — it's idempotent.

**All you need is an LLM CLI subscription.** Evolve drives the CLIs you're already signed into — your Claude, Codex, or Gemini subscription is enough to run the loop.

Everything else has sane defaults; the only knobs live in `.evolve/policy.json` and are all optional:

| Setting | Default | What it does |
|---|---|---|
| per-phase model pins | all-Claude | which LLM + model runs each phase (written by `/evo:setup`) |
| `workflow.cycle_budget` | `enforce` | `enforce` = advisor decides the cycle count; `off` = omitting `--cycles` runs a single cycle |
| `workflow.max_cycles_cap` | `25` | safety ceiling for how many cycles the advisor may run |

Most people run `/evo:setup` once and never open the file.

### Run several features at once

Give each feature its own loop and run them **in parallel** — each in its own git worktree and branch, each on its own LLM. They don't step on each other, and only green changes merge to `main`, one at a time.

```bash
# Fan a backlog into N concurrent, file-disjoint cycles (CLI-native):
evolve fleet --count 3 --goal-hash <hash> --plan backlog.json
```

Or run a separate `/evo:loop` in each of several git worktrees — one per feature, each on the CLI you prefer (Claude · Codex · Gemini). Either way, isolation is by **worktree + branch**, so concurrent loops can't corrupt each other; the merge to `main` is **serialized** for safety. It's safe parallelism across **independent** work — bounded by your machine and your LLM rate limits, not magic infinite scale.

---

## The problem it solves

Autonomous agents are now good enough to write a feature, fix a bug, or refactor a module unattended. So the bottleneck moved. It's no longer *can the agent write the code?* — it's *can you merge what it wrote without re-reading every diff?*

The industry's default answer is "ask another LLM if it looks done." That judge is the weakest link in the whole system. A single model grading code — usually a sibling of the one that wrote it — shares the same blind spots, prefers its own style, and rewards confident prose over correctness. It will happily tell you a hallucinated test suite passes. Worse, an agent under pressure to "finish" learns to *game* whatever judge you give it: stubbing the failing test, narrowing the assertion, writing an `echo PASS` script, declaring victory in the summary while the code is broken.

Evolve Loop is built around that exact failure. Two things make it different from a normal CI run or a single-agent loop:

- **The reviewer is a different model family from the author**, and it's prompted to *refute* the work, not bless it.
- **The merge gate is deterministic code, not a model's opinion.** The verdict is a set of executable checks whose exit codes *are* the decision. The model can write eloquent prose all day; nothing ships unless the checks come back green.

---

## What it does best

If you only read one section, read this. These are the things Evolve Loop is built to do better than a plain agent loop or a single long-running skill:

- **Accomplish long, multi-step work by splitting it into isolated phases.** A big task that would overwhelm one prompt is decomposed into a fixed spine of small, independent stages — each with one job, its own context, and a single artifact it must produce. Complexity is bounded per phase, not per task.
- **Catch the AI gaming its own success criteria.** Adversarial cross-family review, deterministic verdicts, mutation testing that rejects fake tests, and a tamper-evident ledger together make "lie about being done" structurally hard — not just discouraged by a prompt.
- **Get smarter every run.** Failures are distilled into lesson files that are fed back into the next cycle's planning. Mistakes don't repeat; the system compounds.
- **Survive long unattended runs.** Quota walls, rate limits, and context-window failures are expected. Work in flight is checkpointed and resumable rather than discarded.
- **Stay vendor-flexible.** Route Scout to Gemini, the builder to Claude Sonnet, the auditor to Claude Opus — whatever mix you trust — without changing the pipeline.

It is **not** a benchmark-chasing code-writing agent. It's the governance and trust layer you put *around* one.

---

## How it works: isolated phases, artifacts as the only interface

Every cycle runs the same spine of phases:

```
INTENT → SCOUT → TRIAGE → [PLAN-REVIEW] → [TDD] → BUILD → AUDIT → SHIP → LEARN
```

- **Intent** turns a vague goal into a structured spec (goals, non-goals, constraints, acceptance criteria) and is required to challenge at least one premise.
- **Scout** explores the codebase, does any needed research, and proposes work.
- **Triage** bounds *this* cycle's scope — what to do now, defer, or drop.
- **Plan-Review** *(optional)* fans the plan out to four lenses (product, engineering, design, security) before any code is written.
- **TDD** *(optional, on by default)* writes failing tests that encode the acceptance criteria — written by a *separate* agent from the one that will implement.
- **Build** implements the change in an isolated git worktree.
- **Audit** adversarially reviews the work and runs the deterministic check suite.
- **Ship** commits only if the verdict is green.
- **Learn** captures a carryover note on success, or a structured failure lesson on failure.

The important part isn't the list of phases — it's how they're wired together.

**Phases communicate only through artifacts.** Each phase reads the file(s) the previous phase wrote and writes exactly one output file for the next. Intent writes a spec; Scout reads it and writes a report; the builder reads that and writes a build report plus the check suite; the auditor reads those and writes a verdict. There is no shared mutable state a phase can reach into, no hidden channel, no "the model just remembers." If it isn't in the artifact, the next phase doesn't see it.

**Each phase runs in isolation.** It gets its own context window, can run on its own model, and — for the agents that write code — its own git worktree on a throwaway branch. A phase can't see another phase's scratch work, can't edit files outside its lane, and can't reach forward or backward in the pipeline. This is what makes long tasks tractable: every stage is a small, well-scoped problem with a defined input and a defined output, not a sprawling conversation that drifts as it grows.

**The phases can't be skipped or reordered.** Phase ordering, write-path scoping, and the ship gate are enforced by the runtime at the OS layer — not by asking the model nicely in a prompt. A model can be wrong, biased, or actively adversarial, but it cannot reorder phases, write outside its worktree, or ship without a green verdict, because those aren't instructions; they're code.

That last point is the whole design philosophy in one line: **LLMs do the qualitative work; deterministic code owns every gate.** Deciding *what* to build, *how* to build it, and *what looks wrong* are judgment calls only a model does well. Phase ordering, scope enforcement, the ship verdict, and the audit trail are mechanical — so they live in code, where the model gets no vote.

### A cycle, end to end

Say you run `/evo:loop --cycles 1 "make the export endpoint resilient to large payloads."` Each step below reads only the artifact the step before it produced:

- **Intent** restates the goal as a spec — and pushes back: *is "large" 10 MB or 10 GB? Is streaming acceptable, or must the response stay synchronous?* It records the assumptions it's proceeding on.
- **Scout** reads the spec, traces the endpoint through the codebase, notices the current in-memory buffering, and proposes a fix.
- **Triage** decides the streaming rewrite is in scope for this cycle, and defers the unrelated retry-logic cleanup it also spotted.
- **TDD** writes failing tests first: a 2 GB payload must not exhaust memory; the response must stay correct.
- **Build**, in its own throwaway worktree, implements streaming until those tests pass — and can touch nothing outside its lane.
- **Audit**, on a different model family, tries to *break* it: edge cases, regressions, and whether those tests actually test anything. Then it runs the check suite.
- **Ship** sees a green verdict and commits. (A red verdict routes the cycle to a retrospective instead of merging.)
- **Learn** records what carried over for next time.

No step takes the previous step's *word* for anything — it reads the file. That's why the same machinery handles a one-line fix and a multi-file refactor without changing shape: the work is always a chain of small, isolated stages with a defined input and a defined output.

---

## Catching the AI when it games the system

The threat isn't a malicious human. It's the LLM doing what LLMs do under pressure to look successful: confabulating "looks done" verdicts, hallucinating evidence, and taking the path of least resistance to a green checkmark. Evolve Loop answers this structurally, in three layers.

### Layer 1 — Structural integrity (always on, no bypass)

Three guards sit between the agents and anything irreversible:

- **Phase guard** — blocks out-of-order or skipped phases.
- **Role guard** — blocks any write outside the current phase's allowed paths (the builder can't edit the gates that grade it; the auditor can't touch source at all — its repo is mounted read-only).
- **Ship guard** — blocks any commit or push that didn't go through the sanctioned ship path with a green verdict.

Behind them is a **tamper-evident ledger**: an append-only log where each entry hashes the previous one. Altering any past entry breaks every hash after it, so the cycle's history can't be quietly rewritten. This layer has no off switch.

### Layer 2 — OS isolation

Every model subprocess can run inside an OS sandbox (`sandbox-exec` on macOS, `bwrap` on Linux), and read-only roles get the repo bind-mounted read-only. Each building cycle gets its own git worktree on a temporary branch, so one cycle's edits can't leak into another's.

### Layer 3 — Anti-gaming workflow defaults

This is where the actual *detection* of gaming lives. Each default targets a specific way a single-model loop gets fooled:

| How an AI fakes success | How Evolve Loop catches it |
|---|---|
| **Self-preference / sibling bias** — a judge sharing the author's model family shares its blind spots and rubber-stamps. | The auditor runs on a **different model family** from the builder, and is told to refute rather than approve. |
| **Style over substance** — judges reward confident, verbose, well-formatted prose. | The ship verdict is **not prose**. It's the exit codes of an executable check suite. Eloquence earns nothing. |
| **Tautological tests** — the agent writes a check that can't fail (`echo PASS`, assert-true, grep-for-its-own-output). | **Mutation testing** rejects checks that still pass when the code is deliberately broken. |
| **Compounding hallucination** — fake code *and* fake tests reinforce each other. | Tests are written **before** the code, by a **separate agent**, so the implementer can't author its own bar. |
| **Evidence fabrication** — the agent cites files it didn't touch or claims work it didn't do. | Scout claims are checked against the real diff; audit citations are bound to files actually in this cycle's change set. |
| **Narrative drift** — the summary says one thing; the ledger says another. | Key report sections are auto-rendered from ledger truth, not from model prose, so the story can't diverge from what happened. |

The net effect: the model is free to *attempt* any of these. It just can't make them ship.

### …and rectifies it

Detection is only half of it. When a cycle fails — including when the framework catches *itself* gaming — a retrospective agent reads the wreckage and writes a structured root-cause lesson with a concrete prevention rule. That lesson is verified to exist on disk and merged into the project's long-term memory. The next cycle's planning reads it automatically. Over time, whole *classes* of gaming get structural fixes, not just one-off patches (see the incidents below).

Deep dive: [docs/concepts/trust-architecture.md](docs/concepts/trust-architecture.md) — the full threat model and gaming-pattern catalog.

---

## Failures become lessons (memory that compounds)

Most agent loops have, at best, single-loop learning: detect an error, retry the action. Evolve Loop adds the second loop — detect the error, question the *assumption* that caused it, and change the policy so it can't recur.

When a cycle fails, the retrospective writes a lesson like:

> "When adding a new state field a phase depends on, grep every regression check that reads that phase's inputs and update the fixtures." — distilled from the cycle where forgetting exactly that broke the run.

That lesson is durable. The cycle that learned it shipped long ago; the rule still fires in every future planning pass. Dozens of such lessons accumulate — some authored 60+ cycles back and still relevant — so the system genuinely gets harder to fool the longer you run it. This is a multi-agent take on the Reflexion pattern (Shinn et al., 2023), with one deliberate twist: the *verdict* that triggers reflection is never a model claim — it's the deterministic check suite. The auditor can write prose; it can't fake the gate that decides whether a lesson was needed.

Deep dive: [docs/concepts/self-evolution.md](docs/concepts/self-evolution.md).

---

## Built to survive long runs

Long unattended cycles fail routinely — subscription quotas exhaust, APIs return errors, models hit context limits. The contract is simple: **work in flight survives common failures.** A failed cycle's reasoning becomes a lesson; a quota wall mid-cycle checkpoints the worktree and state so a single `--resume` picks up where it stopped; recoverable failures preserve the builder's edits rather than discarding them. The canonical motivating incident — 30 minutes of build work lost to a quota wall mid-audit — is exactly what checkpoint-resume now prevents.

---

## How it compares

Most "autonomous" coding tools collapse to one model doing everything and grading its own homework:

```
   ┌─────────── same model (or same family) ───────────┐
USER →  write code  →  "does this look done?"  →  ship
   └────────────── one judgment, one blind spot ───────┘
```

Evolve Loop splits the roles and hands the *final* verdict to code:

```
USER → Scout → (model A writes failing tests) → (model B writes code)
                                                      │
                  (model C, different family, told to refute) audits
                                                      │
                       deterministic check suite — green or it doesn't ship
                                                      │
                            ship → a durable lesson is written
```

**Versus other long-running agent skills** (`/goal`, self-evolving/self-improving agents, skill frameworks): those typically gate on a small validator LLM or skill-described exit criteria, keep memory in conversation or a notes file, run on one CLI, and treat anti-gaming as convention. Evolve Loop's verdict is deterministic check exit codes, its memory is durable lesson files fed back into planning, it routes across Claude/Gemini/Codex per phase, and its anti-gaming is structural at three layers.

**Versus the famous code-writing agents** (Devin, OpenHands, SWE-agent, Aider): those optimize *how well the agent solves the task*, usually benchmarked on SWE-bench. Evolve Loop sits on a different axis — it's a **trust-and-governance pipeline** that can *drive* those agents and adds the verification, learning, and recovery layer on top. It isn't competing on raw coding ability; it's deciding whether the code is safe to merge **unattended**. Pick by your single biggest constraint: human-in-the-loop control (Aider), hands-off autonomy (Devin/OpenHands), or **unattended trust** (Evolve Loop).

### The honest tradeoffs

Evolve Loop is **not** always the right choice:

- **Higher friction.** A full phase spine per cycle runs ~15–30 minutes of wall-clock; adversarial mode (two model families) costs more per cycle than a single-model loop. A plain `/goal` loop is faster and simpler.
- **Steeper learning curve.** The trust kernel, the check suite, the router, and the recovery model are concepts you have to absorb.
- **Optimized for trust, not speed.** Fastest autonomous coding is a single-agent loop. *Safest* merge is this.

Best fit: teams or solo developers running long unattended cycles on production code, where a bad merge is expensive. Full comparison: [docs/comparisons/long-running-claude-skills.md](docs/comparisons/long-running-claude-skills.md).

---

## Proof: the framework caught its own bugs

The strongest evidence that the anti-gaming is real is that it has repeatedly caught *itself*.

- **The model-swap experiment.** Routing a different model to Scout and the builder shipped a damaged cycle — and exposed seven distinct bugs in the framework, several caught by its own audit and grounding checks. All seven got structural fixes (grounding verification, audit-citation binding, ledger-truth report rendering) over the next two cycles.
- **A trust-kernel ordering breach.** A worktree commit briefly reached `main` before the post-audit gate could verify it, exposing a short ordering window. Commit self-attestation and a pre-merge tree verification shipped within hours.
- **An autonomous goal divergence.** Triage chose to ship something other than the operator's stated plan — and was right: the stated item was already done, so the deviation saved a wasted cycle. (Operator goal text is *input*, not an unquestionable directive — by design.)

The value proposition was never "cycles never fail." It's that failures **produce durable lessons and structural fixes** — which is the most valuable kind of bug report. Case studies: [docs/incidents/](docs/incidents/).

---

## Learn more

The README is the surface; the depth lives in `docs/`:

- **Concepts (teaching-first):** [overview](docs/concepts/overview.md) · [self-evolution](docs/concepts/self-evolution.md) · [trust architecture](docs/concepts/trust-architecture.md) · [error recovery](docs/concepts/error-recovery.md) · [pluggability](docs/concepts/pluggability.md)
- **Architecture (reference-first):** [docs/architecture/](docs/architecture/) — per-phase mechanics, the check-suite format, the routing kernel, the checkpoint-resume protocol.
- **Incidents (case studies):** [docs/incidents/](docs/incidents/) — the self-caught bugs, in forensic detail.
- **Decisions:** [docs/architecture/adr/](docs/architecture/adr/) — every architectural choice, with context and consequence.
- **See it visually:** the **[landing page](https://mickeyyaya.github.io/evolve-loop/)** — the whole pitch (bottleneck → pillars → self-caught incident → quick start) as one scrollable page.

---

## Contributing

Contributions welcome. The project is itself run by Evolve Loop — every commit on `main` is either an operator's reviewed manual ship or an automated cycle ship with a full audit trail. Start with [CLAUDE.md](CLAUDE.md) (the runtime contract) and [AGENTS.md](AGENTS.md) (cross-CLI invariants).

If you find a gaming pattern the framework didn't catch, please file an issue with the cycle number, the relevant reports, and what you expected versus what shipped — those are the most valuable reports we get.

---

## Version

**Current (v21.7)** — full release history in [CHANGELOG.md](CHANGELOG.md). Releases are cut via `evolve release X.Y.Z`.

| Version | Date | Notes |
|---|---|---|
| v20.4 | Jun 24 | see [CHANGELOG.md](CHANGELOG.md) |
| v21.0 | Jun 24 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v21.1 | Jun 24 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v21.2 | Jun 26 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v21.3 | Jun 26 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v21.4 | Jun 29 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v21.5 | Jun 30 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v21.6 | Jul 2 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v21.7 | Jul 3 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |

---

## License & Links

- **License:** Apache-2.0 — see [LICENSE](LICENSE) (third-party notices in [NOTICE](NOTICE))
- **GitHub:** [github.com/mickeyyaya/evolve-loop](https://github.com/mickeyyaya/evolve-loop)
- **Install:** `curl -fsSL https://mickeyyaya.github.io/evolve-loop/install.sh | sh` — or `/plugin marketplace add mickeyyaya/evolve-loop` in Claude Code
- **Changelog:** [CHANGELOG.md](CHANGELOG.md)

The design draws on Reflexion (Shinn et al., 2023), double-loop learning (Argyris & Schön, 1978), the reward-hacking literature (Skalse et al., 2022; Weng, 2024), and the LLM-as-judge bias research that motivates multi-annotator and adversarial evaluation. Full bibliography: [docs/architecture/phase-architecture-citations.md](docs/architecture/phase-architecture-citations.md).
